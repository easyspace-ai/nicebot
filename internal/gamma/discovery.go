package gamma

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"limitorderbot/internal/models"
)

type Discovery struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Discovery {
	return &Discovery{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *Discovery) DiscoverBTC15mMarkets(ctx context.Context) ([]models.Market, error) {
	var out []models.Market
	tsList := generate15MinTimestamps(time.Now(), 48)
	for _, ts := range tsList {
		slug := fmt.Sprintf("btc-updown-15m-%d", ts)
		ev, err := d.fetchEventBySlug(ctx, slug)
		if err != nil {
			continue
		}
		m, ok := parseMarket(ev)
		if ok {
			out = append(out, m)
		}
	}
	// sort by start
	sortMarketsByStart(out)
	return out, nil
}

func generate15MinTimestamps(now time.Time, count int) []int64 {
	// Round down to nearest 15-min mark, then start from next interval.
	t := now.Truncate(time.Minute).Add(-time.Duration(now.Minute()%15) * time.Minute)
	var ts []int64
	for i := 0; i < count; i++ {
		f := t.Add(time.Duration(15*(i+1)) * time.Minute)
		ts = append(ts, f.Unix())
	}
	return ts
}

func (d *Discovery) fetchEventBySlug(ctx context.Context, slug string) (map[string]any, error) {
	u := d.BaseURL + "/events?slug=" + url.QueryEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gamma status=%d", resp.StatusCode)
	}
	var arr []any
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return nil, err
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("not found")
	}
	m, ok := arr[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected gamma response")
	}
	return m, nil
}

func parseMarket(eventOrMarket map[string]any) (models.Market, bool) {
	// Mimic python _parse_market: event response contains markets[].
	var actual map[string]any
	var marketSlug string
	var question string
	var conditionID string

	if marketsRaw, ok := eventOrMarket["markets"]; ok {
		if marketsArr, ok := marketsRaw.([]any); ok && len(marketsArr) > 0 {
			first, _ := marketsArr[0].(map[string]any)
			actual = first
			marketSlug = asString(eventOrMarket["slug"])
			q := asString(first["question"])
			if q == "" {
				q = asString(eventOrMarket["title"])
			}
			question = q
			conditionID = asString(first["conditionId"])
		}
	}
	if actual == nil {
		actual = eventOrMarket
		conditionID = asString(eventOrMarket["conditionId"])
		if conditionID == "" {
			conditionID = asString(eventOrMarket["condition_id"])
		}
		marketSlug = asString(eventOrMarket["slug"])
		if marketSlug == "" {
			marketSlug = asString(eventOrMarket["market_slug"])
		}
		question = asString(eventOrMarket["question"])
		if question == "" {
			question = asString(eventOrMarket["title"])
		}
	}
	if conditionID == "" || marketSlug == "" || question == "" {
		return models.Market{}, false
	}

	startTS, endTS := extractStartEnd(marketSlug, actual, eventOrMarket)
	if startTS == 0 || endTS == 0 {
		return models.Market{}, false
	}

	outcomes := parseOutcomes(actual, eventOrMarket)
	isActive := asBool(eventOrMarket["active"])
	isResolved := asBool(eventOrMarket["closed"]) || asBool(eventOrMarket["resolved"])

	return models.Market{
		ConditionID: conditionID,
		MarketSlug:  marketSlug,
		Question:    question,
		StartTS:     startTS,
		EndTS:       endTS,
		Outcomes:    outcomes,
		IsActive:    isActive,
		IsResolved:  isResolved,
	}, true
}

func extractStartEnd(slug string, actual map[string]any, event map[string]any) (int64, int64) {
	if strings.Contains(strings.ToLower(slug), "btc-updown-15m-") {
		parts := strings.Split(slug, "btc-updown-15m-")
		if len(parts) > 1 {
			rest := parts[len(parts)-1]
			tsStr := strings.Split(rest, "-")[0]
			if ts, err := parseInt64(tsStr); err == nil {
				return ts, ts + 15*60
			}
		}
	}
	// Fallback iso fields
	startTS := parseISO(asString(actual["startDate"]))
	if startTS == 0 {
		startTS = parseISO(asString(event["start_date"]))
	}
	endTS := parseISO(asString(actual["endDate"]))
	if endTS == 0 {
		endTS = parseISO(asString(event["end_date"]))
	}
	return startTS, endTS
}

func parseOutcomes(actual map[string]any, event map[string]any) []models.Outcome {
	// Prefer clobTokenIds + outcomes
	var outs []models.Outcome
	if raw, ok := actual["clobTokenIds"]; ok && raw != nil {
		var tokenIDs []any
		switch t := raw.(type) {
		case string:
			_ = json.Unmarshal([]byte(t), &tokenIDs)
		case []any:
			tokenIDs = t
		}
		var outcomeNames []any
		if oRaw, ok := actual["outcomes"]; ok {
			switch ot := oRaw.(type) {
			case string:
				_ = json.Unmarshal([]byte(ot), &outcomeNames)
			case []any:
				outcomeNames = ot
			}
		}
		if len(outcomeNames) == 0 {
			outcomeNames = []any{"Up", "Down"}
		}
		for i, id := range tokenIDs {
			name := fmt.Sprintf("Outcome%d", i)
			if i < len(outcomeNames) {
				name = asString(outcomeNames[i])
			}
			outs = append(outs, models.Outcome{TokenID: asString(id), Outcome: name})
		}
	}
	if len(outs) > 0 {
		return outs
	}
	// Fallback tokens field
	if toks, ok := event["tokens"].([]any); ok {
		for _, tok := range toks {
			m, _ := tok.(map[string]any)
			outs = append(outs, models.Outcome{
				TokenID: asString(m["token_id"]),
				Outcome: asString(m["outcome"]),
			})
		}
	}
	if toks, ok := actual["tokens"].([]any); ok {
		for _, tok := range toks {
			m, _ := tok.(map[string]any)
			outs = append(outs, models.Outcome{
				TokenID: asString(m["token_id"]),
				Outcome: asString(m["outcome"]),
			})
		}
	}
	return outs
}

func sortMarketsByStart(markets []models.Market) {
	// insertion sort is fine (<=48)
	for i := 1; i < len(markets); i++ {
		j := i
		for j > 0 && markets[j-1].StartTS > markets[j].StartTS {
			markets[j-1], markets[j] = markets[j], markets[j-1]
			j--
		}
	}
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.ToLower(t) == "true"
	default:
		return false
	}
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not int")
		}
		n = n*10 + int64(ch-'0')
	}
	return n, nil
}

func parseISO(s string) int64 {
	if s == "" {
		return 0
	}
	// python: datetime.fromisoformat(iso.replace("Z","+00:00"))
	s = strings.ReplaceAll(s, "Z", "+00:00")
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// try without seconds
		if t2, err2 := time.Parse("2006-01-02T15:04:05-07:00", s); err2 == nil {
			return t2.Unix()
		}
		return 0
	}
	return t.Unix()
}
