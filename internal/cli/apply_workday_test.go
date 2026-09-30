package cli_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/testutil"
	"github.com/thedavidweng/jobs-cli/v2/internal/workday"
)

func TestWorkdayBrowserInspectAndFillAtReview(t *testing.T) {
	executable := os.Getenv("JOBS_TEST_BROWSER")
	if executable == "" {
		t.Skip("browser fixtures require JOBS_TEST_BROWSER; see docs/browser-testing.md")
	}
	for _, employer := range []string{"bank-a", "bank-b"} {
		t.Run(employer, func(t *testing.T) {
			page := fmt.Sprintf(`<h1 data-automation-id="applyFlowPage">My Information</h1><label for="first">%s given name</label><input id="first" data-automation-id="firstName" required><label for="email">Email</label><input id="email" data-automation-id="email" required><button data-automation-id="bottom-navigation-next-button" onclick="document.querySelector('h1').textContent='Review'; this.remove(); document.body.insertAdjacentHTML('beforeend','<button data-automation-id=submitButton onclick=receipt()>Submit</button>')">Next</button><script>function receipt(){document.body.innerHTML='<h1 data-automation-id="applicationConfirmation">Application submitted</h1><span data-automation-id="applicationId">R-42</span>'}</script>`, employer)
			if employer == "bank-b" {
				page = `<label for="phone">Mobile phone</label><input id="phone" data-automation-id="phoneNumber" required>` + page
			}
			page = `<div data-automation-id="workExperienceSection"><div data-automation-id="workExperience"><input data-automation-id="company" required><input data-automation-id="jobTitle" required><input data-automation-id="startDate" type="month"></div><button data-automation-id="addButton" onclick="const row=this.parentNode.querySelector('[data-automation-id=workExperience]').cloneNode(true);for(const input of row.querySelectorAll('input'))input.value='';this.before(row)">Add work</button></div><div data-automation-id="educationSection"><div data-automation-id="education"><input data-automation-id="school" required><select data-automation-id="degree" required><option value=""></option><option value="BSc">Bachelor</option></select></div></div><label for="authorization">` + employer + ` Work authorization</label><select id="authorization" required><option value=""></option><option value="yes">Yes</option><option value="no">No</option></select>` + page
			page = `<div id="details"><label for="website">Website</label><input id="website" data-automation-id="website" value="saved-old"><label for="country">Country</label><select id="country" data-automation-id="country" required><option value=""></option><option value="CA">Canada</option></select>` + page + `<label for="resume">Resume</label><input id="resume" data-automation-id="resume" type="file" required></div><script>
   const next=document.querySelector('[data-automation-id="bottom-navigation-next-button"]');let stage=0;
   document.body.prepend(document.querySelector('h1'));document.body.append(next);
   next.onclick=()=>{if(stage++===0){window.savedName=document.querySelector('#first').value;window.savedWebsite=document.querySelector('#website').value;window.savedRows=document.querySelectorAll('[data-automation-id=workExperience]').length;document.querySelector('#details').remove();document.querySelector('h1').textContent='Questions';document.body.insertAdjacentHTML('beforeend','<label for="consent">I agree to '+` + fmt.Sprintf("%q", employer) + `+' processing my application</label><input id="consent" type="checkbox" required><fieldset><legend>Sponsorship needed?</legend><label><input type="radio" name="sponsorship" value="yes" required>Yes</label><label><input type="radio" name="sponsorship" value="no" required>No</label></fieldset>');}else{document.querySelector('h1').textContent='Review';next.remove();document.body.insertAdjacentHTML('beforeend','<p id="review">'+window.savedName+'</p><button data-automation-id="submitButton" onclick="receipt()">Submit</button>')}}
   </script>`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, page) }))
			defer server.Close()
			launch := launcher.New().Bin(executable).Headless(true).Leakless(false)
			endpoint, err := launch.Launch()
			if err != nil {
				t.Fatal(err)
			}
			defer launch.Cleanup()
			defer launch.Kill()
			connection := rod.New().Context(context.Background()).ControlURL(endpoint)
			if err := connection.Connect(); err != nil {
				t.Fatal(err)
			}
			ctx, err := connection.Page(proto.TargetCreateTarget{URL: server.URL + "/" + employer + "/site/job/JR1"})
			if err != nil {
				t.Fatal(err)
			}
			if err := ctx.WaitLoad(); err != nil {
				t.Fatal(err)
			}
			tabID := string(ctx.TargetID)
			target := &domain.ApplicationTarget{URL: server.URL + "/" + employer + "/site/job/JR1", Provider: domain.ProviderWorkday, Tenant: employer, Site: "site", ProviderJobID: "JR1"}
			reg := testutil.NewRegistry(map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{Job: fakeJob(domain.SourceIndeed, "1")}}, &testutil.FakeResolver{Target: target}, map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderWorkday: workday.NewProvider()})
			h := newHarness(t).useRegistry(reg)
			manifest := h.writeFile("candidate.json", `{"candidate":{"first_name":"Ada","email":"ada@example.com","phone":"+1 555 0100","website":"","address":{"country":"CA"},"work":[{"name":"Engine","position":"Programmer","startDate":"1842-01"},{"name":"Analytical","position":"Writer","startDate":"1843-02"}],"education":[{"institution":"University","studyType":"Bachelor"}]},"answers":[{"question_id":"authorization","value":"yes"},{"question_id":"consent","value":true},{"question_id":"sponsorship","value":"No"}]}`)
			flags := []string{"--json", "apply", "--browser-endpoint", endpoint, "--browser-tab", tabID, "--state", h.dir + "/state.json"}
			run := func(args ...string) (string, int) {
				out, _, code := h.run(append(append([]string{}, flags...), args...)...)
				return out, code
			}
			resume := h.writeFile("resume.pdf", "fixture resume")
			path := h.dir + "/application.json"
			var out string
			var code int
			if employer == "bank-a" {
				if err := fixtureEvaluate(ctx, `document.querySelector('#details').hidden=true;document.querySelector('h1').hidden=true;const start=document.createElement('button');start.setAttribute('data-automation-id','adventureButton');start.textContent='Start application';start.onclick=()=>{document.querySelector('#details').hidden=false;document.querySelector('h1').hidden=false;start.remove()};document.body.prepend(start)`, nil); err != nil {
					t.Fatal(err)
				}
				out, code = run("prepare", "indeed:1", "--manifest", manifest, "--resume", resume, "--out", path)
				if code != 0 {
					t.Fatalf("prepare pending start: %s", out)
				}
				preparedDoc := decodeEnvelope(t, out)
				artifactDoc, ok := preparedDoc.Data["artifact"].(map[string]any)
				if !ok || artifactDoc["pending_action"] != "start_application_required" {
					t.Fatalf("start pending: %s", out)
				}
				out, code = run("fill", "--artifact", path, "--confirm")
				if code != 0 || decodeEnvelope(t, out).Data["status"] != "requirements_changed" {
					t.Fatalf("start fixture: %s", out)
				}
			}
			out, code = run("prepare", "indeed:1", "--manifest", manifest, "--resume", resume, "--out", path)
			if code != 0 {
				t.Fatalf("prepare: %s", out)
			}
			var value string
			if err := fixtureEvaluate(ctx, `document.querySelector("#first").value`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("prepare mutated the page")
			}
			out, code = run("fill", "--artifact", path, "--confirm", "--state", h.dir+"/missing/state.json")
			if code == 0 {
				t.Fatalf("unwritable state accepted: %s", out)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector("#first").value`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("unwritable state allowed writes")
			}
			out, code = run("fill", "--artifact", path, "--confirm", "--read-only")
			docRead := decodeEnvelope(t, out)
			requireCode(t, &docRead, "READ_ONLY_VIOLATION", 4, code)
			if err := fixtureEvaluate(ctx, `document.body.insertAdjacentHTML('afterbegin','<div data-automation-id="signInForm">Sign in</div>')`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "login_required" {
				t.Fatalf("login pause: %s", out)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('[data-automation-id="signInForm"]').remove()`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path)
			doc := decodeEnvelope(t, out)
			requireCode(t, &doc, "CONFIRMATION_REQUIRED", 10, code)
			out, code = run("fill", "--artifact", path, "--dry-run")
			if code != 0 || decodeEnvelope(t, out).Data["dry_run"] != true {
				t.Fatalf("dry run: %s", out)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('label[for=authorization]').textContent='Changed authorization wording'`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "requirements_changed" {
				t.Fatalf("stale questions: %s", out)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector("#first").value`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("stale artifact mutated browser")
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('label[for=authorization]').textContent=`+fmt.Sprintf("%q", employer+" Work authorization"), nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "requirements_changed" {
				t.Fatalf("new questionnaire: %s", out)
			}
			out, code = run("prepare", "indeed:1", "--manifest", manifest, "--resume", resume, "--previous-artifact", path, "--out", path)
			if code != 0 {
				t.Fatalf("prepare new step: %s", out)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "review_ready" {
				t.Fatalf("fill: %s", out)
			}
			if err := fixtureEvaluate(ctx, `window.savedName`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "Ada" {
				t.Fatalf("reviewed name: %s", value)
			}
			if err := fixtureEvaluate(ctx, `window.savedWebsite`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("explicit empty website retained saved value")
			}
			var rows int
			if err := fixtureEvaluate(ctx, `window.savedRows`, &rows); err != nil {
				t.Fatal(err)
			}
			if rows != 2 {
				t.Fatalf("work rows: %d", rows)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('input[name=sponsorship]:checked')?.value||''`, &value); err != nil {
				t.Fatal(err)
			}
			if value != "no" {
				t.Fatalf("radio label was not filled: %s", value)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('input[name=sponsorship][value=yes]').click()`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("submit", "--artifact", path, "--confirm")
			radioChanged := decodeEnvelope(t, out)
			requireCode(t, &radioChanged, "VALIDATION_FAILED", 7, code)
			if err := fixtureEvaluate(ctx, `document.querySelector('input[name=sponsorship][value=no]').click()`, nil); err != nil {
				t.Fatal(err)
			}
			if err := fixtureEvaluate(ctx, `document.querySelector('#consent').required=false`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("submit", "--artifact", path, "--confirm")
			requiredChanged := decodeEnvelope(t, out)
			requireCode(t, &requiredChanged, "VALIDATION_FAILED", 7, code)
			if err := fixtureEvaluate(ctx, `document.querySelector('#consent').required=true`, nil); err != nil {
				t.Fatal(err)
			}
			h.writeFile("resume.pdf", "changed document")
			out, code = run("submit", "--artifact", path, "--confirm")
			changedFile := decodeEnvelope(t, out)
			requireCode(t, &changedFile, "APPLICATION_INCOMPLETE", 7, code)
			h.writeFile("resume.pdf", "fixture resume")
			if err := fixtureEvaluate(ctx, `document.querySelector('#review').textContent='Changed applicant'`, nil); err != nil {
				t.Fatal(err)
			}
			out, code = run("submit", "--artifact", path, "--confirm")
			stale := decodeEnvelope(t, out)
			requireCode(t, &stale, "VALIDATION_FAILED", 7, code)
			if err := fixtureEvaluate(ctx, `document.querySelector('#review').textContent='Ada'`, nil); err != nil {
				t.Fatal(err)
			}
			if employer == "bank-b" {
				if err := fixtureEvaluate(ctx, `window.attempts=0;window.receipt=()=>{window.attempts++}`, nil); err != nil {
					t.Fatal(err)
				}
				out, code = run("submit", "--artifact", path, "--confirm")
				if code != 0 || decodeEnvelope(t, out).Data["status"] != "submission_uncertain" {
					t.Fatalf("uncertain: %s", out)
				}
				out, code = run("submit", "--artifact", path, "--confirm")
				if code != 0 || decodeEnvelope(t, out).Data["status"] != "submission_uncertain" {
					t.Fatalf("retry uncertain: %s", out)
				}
				var attempts int
				if err := fixtureEvaluate(ctx, `window.attempts`, &attempts); err != nil {
					t.Fatal(err)
				}
				if attempts != 1 {
					t.Fatalf("submission attempts: %d", attempts)
				}
				return
			}
			out, code = run("submit", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["application_id"] != "R-42" {
				t.Fatalf("submit: %s", out)
			}
		})
	}
}

func fixtureEvaluate(page *rod.Page, expression string, out any) error {
	result, err := page.Eval("expression => eval(expression)", expression)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return result.Value.Unmarshal(out)
}
