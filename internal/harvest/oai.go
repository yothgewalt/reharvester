package harvest

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/yothgewalt/reharvester/internal/paper"
)

const oaiEndpoint = "https://oaipmh.arxiv.org/oai"

// OAI harvests arXiv's OAI-PMH feed, the channel arXiv provides for bulk
// metadata. It cannot search: it lists every record in the sets named by
// Query.Categories (required) that changed since Query.From, and keywords and
// years are applied locally by localSelect. A broad category over many years
// is hundreds of pages at the politeness delay, so keep categories narrow.
type OAI struct {
	c *Client
	// BaseURL overrides the endpoint; tests point it at an httptest server.
	BaseURL string
}

type oaiResponse struct {
	Error struct {
		Code string `xml:"code,attr"`
		Text string `xml:",chardata"`
	} `xml:"error"`
	Records []struct {
		Header struct {
			Status string `xml:"status,attr"`
		} `xml:"header"`
		ArXiv struct {
			ID         string `xml:"id"`
			Created    string `xml:"created"`
			Title      string `xml:"title"`
			Categories string `xml:"categories"`
			Abstract   string `xml:"abstract"`
			Authors    []struct {
				Keyname   string `xml:"keyname"`
				Forenames string `xml:"forenames"`
				Suffix    string `xml:"suffix"`
			} `xml:"authors>author"`
		} `xml:"metadata>arXiv"`
	} `xml:"ListRecords>record"`
	Token string `xml:"ListRecords>resumptionToken"`
}

func (o *OAI) Harvest(ctx context.Context, q Query, prog Progress) ([]paper.Paper, error) {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	cats := nonEmpty(q.Categories)
	if len(cats) == 0 {
		return nil, fmt.Errorf("the oai source needs categories (e.g. physics.flu-dyn): OAI-PMH lists whole categories and cannot search by keyword")
	}
	var sets []string
	for _, c := range cats {
		if s := oaiSet(c); !slices.Contains(sets, s) {
			sets = append(sets, s)
		}
	}
	base := o.BaseURL
	if base == "" {
		base = oaiEndpoint
	}

	sel := newLocalSelect(q)
	scanned := 0
	for _, set := range sets {
		v := url.Values{"verb": {"ListRecords"}, "metadataPrefix": {"arXiv"}, "set": {set}}
		if q.From > 0 {
			v.Set("from", fmt.Sprintf("%d-01-01", q.From))
		}
		for {
			if err := o.c.pause(ctx); err != nil {
				return sel.result(), err
			}
			body, err := o.c.get(ctx, base+"?"+v.Encode(), nil)
			if err != nil {
				return sel.result(), err
			}
			var r oaiResponse
			if err := xml.Unmarshal(body, &r); err != nil {
				return sel.result(), fmt.Errorf("oai: decode: %w", err)
			}
			switch r.Error.Code {
			case "":
			case "noRecordsMatch":
				r.Token = ""
			default:
				return sel.result(), fmt.Errorf("oai: %s: %s", r.Error.Code, strings.TrimSpace(r.Error.Text))
			}
			for _, rec := range r.Records {
				if rec.Header.Status == "deleted" {
					continue
				}
				a := rec.ArXiv
				authors := make([]string, len(a.Authors))
				for i, au := range a.Authors {
					authors[i] = strings.Join([]string{au.Forenames, au.Keyname, au.Suffix}, " ")
				}
				// <created> is the latest version's date, so the id's YYMM is the
				// better submission date.
				published := arxivIDDate(a.ID)
				if published.IsZero() {
					published, _ = time.Parse(time.DateOnly, a.Created)
				}
				if p, ok := newPaper(a.ID, a.Title, a.Abstract, authors, strings.Fields(a.Categories), published, true); ok {
					sel.offer(p)
				}
			}
			scanned += len(r.Records)
			prog(0, sel.Matched, fmt.Sprintf("%s: listed %d records, %d match", set, scanned, sel.Matched))
			if strings.TrimSpace(r.Token) == "" {
				break
			}
			v = url.Values{"verb": {"ListRecords"}, "resumptionToken": {strings.TrimSpace(r.Token)}}
		}
	}
	out := sel.result()
	prog(len(out), sel.Matched, fmt.Sprintf("listed %d records; kept %d of %d matching", scanned, len(out), sel.Matched))
	return out, nil
}

// oaiSet maps an arXiv category to its OAI-PMH set: cs.LG -> cs:cs:LG,
// physics.flu-dyn -> physics:physics:flu-dyn, astro-ph.GA -> physics:astro-ph:GA,
// hep-ph -> physics:hep-ph. Archives outside the top-level groups live under
// physics.
func oaiSet(category string) string {
	archive, sub, _ := strings.Cut(strings.TrimSpace(category), ".")
	group := "physics"
	switch archive {
	case "cs", "math", "stat", "eess", "econ", "q-bio", "q-fin":
		group = archive
	}
	set := group + ":" + archive
	if sub != "" {
		set += ":" + sub
	}
	return set
}
