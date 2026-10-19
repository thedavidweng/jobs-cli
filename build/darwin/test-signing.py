import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('sign-and-notarize.sh')


class SigningTest(unittest.TestCase):
    def test_archive_ids_sign_only_shipped_darwin_executables(self):
        config = json.loads(subprocess.check_output(
            ['ruby', '-ryaml', '-rjson', '-e',
             'puts JSON.generate(YAML.load_file(".goreleaser.yaml", aliases: true))'],
            text=True))
        builds = {build.get('id', 'jobs-cli'): build for build in config['builds']}
        self.assertNotIn('hooks', builds['jobs-cli'])
        self.assertNotIn('hooks', builds['universal-slices'])
        self.assertIn('"{{ .IsSnapshot }}" darwin', builds['darwin-native']['hooks']['post'])
        universal, = config['universal_binaries']
        self.assertEqual(universal['ids'], ['universal-slices'])
        self.assertIn('"{{ .IsSnapshot }}" darwin', universal['hooks']['post'])
        archives = {archive['id']: archive['ids'] for archive in config['archives']}
        self.assertEqual(archives, {'native': ['jobs-cli'], 'darwin-native': ['darwin-native'],
                                    'universal': ['jobs-cli-universal']})
        cask, = config['homebrew_casks']
        self.assertEqual(cask['ids'], ['native', 'universal'])
        self.assertIn('com.apple.quarantine', cask['custom_block'])

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
