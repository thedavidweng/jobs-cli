import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('sign-and-notarize.sh')


class SigningTest(unittest.TestCase):
    def test_archive_verification_checks_each_slice_and_preserves_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            archive = root / 'jobs-cli_darwin_universal.tar.gz'
            archive.touch()
            mock = '''#!/bin/bash
case "$(basename "$0")" in
  tar) touch "$4/jobs-cli" ;;
  lipo) echo 'x86_64 arm64' ;;
  codesign)
    printf '%s\\n' "$*" >> "$CALL_LOG"
    if [[ "$*" == *"--architecture arm64"* ]]; then exit "$ARM_EXIT"; fi
    ;;
esac
'''
            for tool in ['tar', 'lipo', 'codesign']:
                path = root / tool
                path.write_text(mock)
                path.chmod(0o755)
            for exit_code in ['0', '3']:
                calls = root / 'calls'
                calls.write_text('')
                result = subprocess.run(
                    ['bash', str(SCRIPT.with_name('verify-archives.sh')), str(root)],
                    env=dict(os.environ, PATH=f'{root}:{os.environ["PATH"]}',
                             CALL_LOG=str(calls), ARM_EXIT=exit_code),
                    capture_output=True, text=True)
                self.assertEqual(result.returncode, int(exit_code), result.stderr)
                self.assertIn('--architecture x86_64', calls.read_text())
                self.assertIn('--architecture arm64', calls.read_text())
                self.assertEqual(calls.read_text().count('--check-notarization'), 2)

    def run_signing(self, snapshot='false', goos='darwin', **overrides):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            mock = '''#!/bin/bash
printf '%s\\n' "$(basename "$0") $*" >> "$CALL_LOG"
case "$(basename "$0") $1" in
  'codesign --verify') exit "$VERIFY_EXIT" ;;
  'codesign --display') printf '%s\\n' "Authority=$AUTHORITY" "TeamIdentifier=$TEAM" 'Timestamp=Oct 17, 2026' 'CodeDirectory flags=0x10000(runtime)' >&2 ;;
  'xcrun notarytool')
    if [[ "$2" == submit ]]; then
      printf '{"id":"submission-id","status":"%s"}\\n' "$NOTARY_STATUS"
      exit "$NOTARY_EXIT"
    fi
    printf '{"issues":["notary diagnostic"]}\\n'
    ;;
esac
'''
            for tool in ['codesign', 'xcrun', 'ditto']:
                path = root / tool
                path.write_text(mock)
                path.chmod(0o755)
            env = dict(os.environ, PATH=f'{root}:{os.environ["PATH"]}',
                       CALL_LOG=str(root / 'calls'), APPLE_TEAM_ID='TESTTEAM',
                       APPLE_SIGNING_IDENTITY='Developer ID Application: Test (TESTTEAM)',
                       APPLE_SIGNING_KEYCHAIN=str(root / 'signing.keychain-db'),
                       APPLE_API_KEY_ID='TESTKEY', APPLE_API_ISSUER='test-issuer',
                       APPLE_API_KEY_PATH=str(root / 'key.p8'), TEAM='TESTTEAM',
                       AUTHORITY='Developer ID Application: Test (TESTTEAM)',
                       VERIFY_EXIT='0', NOTARY_STATUS='Accepted', NOTARY_EXIT='0')
            env.update(overrides)
            result = subprocess.run(['bash', str(SCRIPT), str(root / 'jobs-cli'), snapshot, goos],
                                    env=env, text=True, capture_output=True)
            calls = root / 'calls'
            return result, calls.read_text() if calls.exists() else ''

    def test_release_is_verified_before_and_after_notarization(self):
        result, calls = self.run_signing()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('--timestamp --options runtime', calls)
        self.assertIn('--keychain', calls)
        self.assertLess(calls.index('codesign --verify'), calls.index('notarytool submit'))
        self.assertIn('--check-notarization -R =notarized', calls)

    def test_invalid_signatures_or_credentials_stop_submission(self):
        for invalid in [dict(TEAM='WRONGTEAM'), dict(AUTHORITY='Apple Distribution: Test'),
                        dict(VERIFY_EXIT='1'), dict(APPLE_API_KEY_ID='')]:
            with self.subTest(invalid=invalid):
                result, calls = self.run_signing(**invalid)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('notarytool submit', calls)

    def test_rejected_submission_fails_release(self):
        result, calls = self.run_signing(NOTARY_STATUS='Invalid')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('--check-notarization', calls)
        self.assertIn('notarytool log submission-id', calls)
        self.assertIn('notary diagnostic', result.stdout)

    def test_failed_submit_prints_result_and_diagnostics_before_failing(self):
        result, calls = self.run_signing(NOTARY_EXIT='1', NOTARY_STATUS='Invalid')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('"status":"Invalid"', result.stdout)
        self.assertIn('notarytool log submission-id', calls)
        self.assertNotIn('--check-notarization', calls)

    def test_native_non_darwin_binaries_do_not_use_apple_credentials(self):
        for goos in ['linux', 'windows']:
            result, calls = self.run_signing(goos=goos, APPLE_API_KEY_ID='')
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(calls, '')

    def test_explicit_acceptance_mode_signs_snapshot(self):
        result, calls = self.run_signing(snapshot='true', APPLE_SIGNING_ACCEPTANCE='true')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('notarytool submit', calls)

    def test_snapshot_does_not_use_signing_credentials(self):
        result, calls = self.run_signing(snapshot='true', APPLE_API_KEY_ID='')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, '')


if __name__ == '__main__':
    unittest.main()
