package cli_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chromedp/chromedp"
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
			page += `<label for="resume">Resume</label><input id="resume" data-automation-id="resume" type="file" required><script>
   const next=document.querySelector('[data-automation-id="bottom-navigation-next-button"]');let stage=0;
   next.onclick=()=>{if(stage++===0){document.querySelector('h1').textContent='Questions';document.body.insertAdjacentHTML('beforeend','<label for="consent">I agree to '+` + fmt.Sprintf("%q", employer) + `+' processing my application</label><input id="consent" type="checkbox" required>');}else{document.querySelector('h1').textContent='Review';next.remove();document.body.insertAdjacentHTML('beforeend','<button data-automation-id="submitButton" onclick="receipt()">Submit</button>')}}
   </script>`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, page) }))
			defer server.Close()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address, ok := listener.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatal("expected TCP address")
			}
			port := address.Port
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			alloc, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(executable), chromedp.Flag("remote-debugging-port", fmt.Sprint(port)))...)
			defer cancel()
			ctx, closeTab := chromedp.NewContext(alloc)
			defer closeTab()
			if err := chromedp.Run(ctx, chromedp.Navigate(server.URL+"/"+employer+"/site/job/JR1")); err != nil {
				t.Fatal(err)
			}
			endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
			tabID := string(chromedp.FromContext(ctx).Target.TargetID)
			target := &domain.ApplicationTarget{URL: server.URL + "/" + employer + "/site/job/JR1", Provider: domain.ProviderWorkday, Tenant: employer, Site: "site", ProviderJobID: "JR1"}
			reg := testutil.NewRegistry(map[domain.Source]domain.SourceAdapter{domain.SourceIndeed: &testutil.FakeSource{Job: fakeJob(domain.SourceIndeed, "1")}}, &testutil.FakeResolver{Target: target}, map[domain.ApplicationProvider]domain.ApplyProvider{domain.ProviderWorkday: workday.NewProvider()})
			h := newHarness(t).useRegistry(reg)
			manifest := h.writeFile("candidate.json", `{"candidate":{"first_name":"Ada","email":"ada@example.com","phone":"+1 555 0100","work":[{"name":"Engine","position":"Programmer","startDate":"1842-01"},{"name":"Analytical","position":"Writer","startDate":"1843-02"}],"education":[{"institution":"University","studyType":"Bachelor"}]},"answers":[{"question_id":"authorization","value":"yes"},{"question_id":"consent","value":true}]}`)
			flags := []string{"--json", "apply", "--browser-endpoint", endpoint, "--browser-tab", tabID, "--state", h.dir + "/state.json"}
			run := func(args ...string) (string, int) {
				out, _, code := h.run(append(append([]string{}, flags...), args...)...)
				return out, code
			}
			resume := h.writeFile("resume.pdf", "fixture resume")
			path := h.dir + "/application.json"
			out, code := run("prepare", "indeed:1", "--manifest", manifest, "--resume", resume, "--out", path)
			if code != 0 {
				t.Fatalf("prepare: %s", out)
			}
			var value string
			if err := chromedp.Run(ctx, chromedp.Value("#first", &value)); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("prepare mutated the page")
			}
			out, code = run("fill", "--artifact", path, "--confirm", "--read-only")
			docRead := decodeEnvelope(t, out)
			requireCode(t, &docRead, "READ_ONLY_VIOLATION", 4, code)
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.body.insertAdjacentHTML('afterbegin','<div data-automation-id="signInForm">Sign in</div>')`, nil)); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "login_required" {
				t.Fatalf("login pause: %s", out)
			}
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('[data-automation-id="signInForm"]').remove()`, nil)); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path)
			doc := decodeEnvelope(t, out)
			requireCode(t, &doc, "CONFIRMATION_REQUIRED", 10, code)
			out, code = run("fill", "--artifact", path, "--dry-run")
			if code != 0 || decodeEnvelope(t, out).Data["dry_run"] != true {
				t.Fatalf("dry run: %s", out)
			}
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('label[for=authorization]').textContent='Changed authorization wording'`, nil)); err != nil {
				t.Fatal(err)
			}
			out, code = run("fill", "--artifact", path, "--confirm")
			if code != 0 || decodeEnvelope(t, out).Data["status"] != "requirements_changed" {
				t.Fatalf("stale questions: %s", out)
			}
			if err := chromedp.Run(ctx, chromedp.Value("#first", &value)); err != nil {
				t.Fatal(err)
			}
			if value != "" {
				t.Fatal("stale artifact mutated browser")
			}
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('label[for=authorization]').textContent=`+fmt.Sprintf("%q", employer+" Work authorization"), nil)); err != nil {
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
			if err := chromedp.Run(ctx, chromedp.Value("#first", &value)); err != nil {
				t.Fatal(err)
			}
			if value != "Ada" {
				t.Fatalf("first name: %s", value)
			}
			var rows int
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('[data-automation-id="workExperience"]').length`, &rows)); err != nil {
				t.Fatal(err)
			}
			if rows != 2 {
				t.Fatalf("work rows: %d", rows)
			}
			h.writeFile("resume.pdf", "changed document")
			out, code = run("submit", "--artifact", path, "--confirm")
			changedFile := decodeEnvelope(t, out)
			requireCode(t, &changedFile, "APPLICATION_INCOMPLETE", 7, code)
			h.writeFile("resume.pdf", "fixture resume")
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('#email').value='changed@example.com'`, nil)); err != nil {
				t.Fatal(err)
			}
			out, code = run("submit", "--artifact", path, "--confirm")
			stale := decodeEnvelope(t, out)
			requireCode(t, &stale, "VALIDATION_FAILED", 7, code)
			if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('#email').value='ada@example.com'`, nil)); err != nil {
				t.Fatal(err)
			}
			if employer == "bank-b" {
				if err := chromedp.Run(ctx, chromedp.Evaluate(`window.attempts=0;window.receipt=()=>{window.attempts++}`, nil)); err != nil {
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
				if err := chromedp.Run(ctx, chromedp.Evaluate(`window.attempts`, &attempts)); err != nil {
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
