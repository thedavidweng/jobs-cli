package linkedinguest

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

const (
	cardClass        = "base-search-card"
	cardLinkClass    = "base-card__full-link"
	cardTitleClass   = "sr-only"
	cardTitleAlt     = "base-search-card__title"
	cardCompanyClass = "base-search-card__subtitle"
	cardLocationCls  = "job-search-card__location"
	cardDateClass    = "job-search-card__listdate"
	cardDateNewClass = "job-search-card__listdate--new"
	entityURNAttr    = "data-entity-urn"
	entityURNPrefix  = "urn:li:jobPosting:"
)

func parseCards(doc *html.Node) []*html.Node {
	var cards []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "div" && hasClass(node, cardClass) {
			cards = append(cards, node)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return cards
}

func jobFromCard(card *html.Node) (domain.Job, *joberrors.Error) {
	id := cardJobID(card)
	title := cardTitle(card)
	if id == "" || title == "" {
		return domain.Job{}, schemaChanged("LinkedIn Guest job card is missing a job id or title")
	}
	job := domain.NewJob(domain.SourceLinkedIn, id)
	job.Title = title
	job.Employer = cardCompany(card)
	job.Location = cardLocation(card)
	job.PostedDate = cardPosted(card)
	job.SourceURL = baseURL + "/jobs/view/" + id
	job.Workplace, job.Remote = workplaceFor(job.Title, job.Location)
	return job, nil
}

func cardJobID(card *html.Node) string {
	if urn := attr(card, entityURNAttr); strings.HasPrefix(urn, entityURNPrefix) {
		if id := strings.TrimSpace(strings.TrimPrefix(urn, entityURNPrefix)); id != "" {
			return id
		}
	}
	href := attr(findElement(card, "a", cardLinkClass), "href")
	if href == "" {
		return ""
	}
	path := href
	if parsed, err := url.Parse(href); err == nil {
		path = parsed.Path
	}
	path = strings.TrimSuffix(path, "/")
	index := strings.LastIndex(path, "-")
	if index < 0 || index == len(path)-1 {
		return ""
	}
	id := path[index+1:]
	if !isDigits(id) {
		return ""
	}
	return id
}

func cardTitle(card *html.Node) string {
	if title := textContent(findElement(card, "span", cardTitleClass)); title != "" {
		return title
	}
	return textContent(findElement(card, "h3", cardTitleAlt))
}

func cardCompany(card *html.Node) string {
	heading := findElement(card, "h4", cardCompanyClass)
	if heading == nil {
		return ""
	}
	if anchor := findElement(heading, "a", ""); anchor != nil {
		if name := textContent(anchor); name != "" {
			return name
		}
	}
	return textContent(heading)
}

func cardLocation(card *html.Node) string {
	return textContent(findElement(card, "span", cardLocationCls))
}

func cardPosted(card *html.Node) string {
	node := findElement(card, "time", cardDateClass)
	if node == nil {
		node = findElement(card, "time", cardDateNewClass)
	}
	return attr(node, "datetime")
}

func workplaceFor(title, location string) (domain.Workplace, bool) {
	text := strings.ToLower(title + " " + location)
	for _, keyword := range []string{"remote", "work from home", "wfh"} {
		if strings.Contains(text, keyword) {
			return domain.WorkplaceRemote, true
		}
	}
	if strings.Contains(text, "hybrid") {
		return domain.WorkplaceHybrid, false
	}
	return domain.WorkplaceUnknown, false
}

func findElement(root *html.Node, tag, class string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found != nil {
			return
		}
		if node.Type == html.ElementNode && (tag == "" || node.Data == tag) && (class == "" || hasClass(node, class)) {
			found = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	if root == nil {
		return nil
	}
	walk(root)
	return found
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func attr(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func hasClass(node *html.Node, class string) bool {
	for _, attribute := range node.Attr {
		if attribute.Key != "class" {
			continue
		}
		for _, token := range strings.Fields(attribute.Val) {
			if token == class {
				return true
			}
		}
	}
	return false
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
