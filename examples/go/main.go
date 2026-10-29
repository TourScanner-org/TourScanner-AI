// Three calls against the TourScanner API, picked by the first argument.
//
//	go run . search <city> [query]
//	go run . availability <activityId> <YYYY-MM-DD> [adults]
//	OPTION_CODE=... go run . book <activityId> <YYYY-MM-DD>
//
// "book" places a real supplier hold and a real Stripe checkout on the
// production host. Point TOURSCANNER_BASE_URL at a sandbox if you have one.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

var (
	base = envOr("TOURSCANNER_BASE_URL", "https://tourscanner.ai")
	key  = os.Getenv("TOURSCANNER_API_KEY")
)

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func do(req *http.Request) []byte {
	req.Header.Set("X-API-Key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		die("%v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		die("%s -> %d %s", req.URL.Path, res.StatusCode, body)
	}
	return body
}

func post(path string, payload any) []byte {
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return do(req)
}

// Prices are minor units: 5400 means 54.00.
func money(cents float64) string { return fmt.Sprintf("%.2f", cents/100) }

type slot struct {
	StartTime  string  `json:"startTime"`
	Available  bool    `json:"available"`
	TotalPrice float64 `json:"totalPrice"`
	Currency   string  `json:"currency"`
	Label      string  `json:"label"`
}

func slotsFor(activityID int, date string) []slot {
	var out struct {
		Slots []slot `json:"slots"`
	}
	json.Unmarshal(post("/api/availability/slots", map[string]any{"activityId": activityID, "date": date, "adults": 2}), &out)
	return out.Slots
}

func search(args []string) {
	city := "rome"
	if len(args) > 0 {
		city = args[0]
	}
	params := url.Values{"city": {city}, "limit": {"5"}}
	if len(args) > 1 {
		params.Set("q", args[1])
	}
	req, _ := http.NewRequest("GET", base+"/api/search/activities?"+params.Encode(), nil)

	var out struct {
		Total      int `json:"total"`
		Activities []struct {
			ID         int      `json:"id"`
			Name       string   `json:"name"`
			ReviewsAvg *float64 `json:"reviews_avg"`
			Price      *struct {
				Currency string  `json:"currency"`
				Price    float64 `json:"price"`
			} `json:"price"`
		} `json:"activities"`
	}
	json.Unmarshal(do(req), &out)

	fmt.Printf("%d activities in %s\n\n", out.Total, city)
	for _, a := range out.Activities {
		price, rating := "no price", "-"
		if a.Price != nil {
			price = money(a.Price.Price) + " " + a.Price.Currency
		}
		if a.ReviewsAvg != nil {
			rating = strconv.FormatFloat(*a.ReviewsAvg, 'f', 1, 64)
		}
		fmt.Printf("%d  %s  (%s stars, %s)\n", a.ID, a.Name, rating, price)
	}
}

func availability(args []string) {
	if len(args) < 2 {
		die("usage: go run . availability <activityId> <YYYY-MM-DD>")
	}
	id, _ := strconv.Atoi(args[0])
	slots := slotsFor(id, args[1])
	if len(slots) == 0 {
		fmt.Println("No slots on that date. Try another one.")
	}
	for _, s := range slots {
		state := "open    "
		if !s.Available {
			state = "sold out"
		}
		fmt.Printf("%s  %s  %s %s  %s\n", s.StartTime, state, money(s.TotalPrice), s.Currency, s.Label)
	}
}

func book(args []string) {
	if len(args) < 2 {
		die("usage: OPTION_CODE=... go run . book <activityId> <YYYY-MM-DD>")
	}
	id, _ := strconv.Atoi(args[0])
	date := args[1]

	// 1. Live slots for the date
	var open *slot
	for _, s := range slotsFor(id, date) {
		if s.Available {
			s := s
			open = &s
			break
		}
	}
	if open == nil {
		die("Nothing open on %s", date)
	}
	fmt.Println("slot:", open.StartTime, money(open.TotalPrice), open.Currency)

	// 2. Options. The hold needs a productOptionCode from here.
	var options map[string]json.RawMessage
	json.Unmarshal(post("/api/booking/options", map[string]any{"activityId": id}), &options)
	fmt.Print("options response keys:")
	for k := range options {
		fmt.Print(" ", k)
	}
	fmt.Println()
	code := os.Getenv("OPTION_CODE")
	if code == "" {
		die("Pick a productOptionCode from the options above and re-run with OPTION_CODE=...")
	}

	// 3. Hold. Reserves the slot and creates the Stripe session.
	var hold struct {
		InternalRef string  `json:"internalRef"`
		RetailPrice float64 `json:"retailPrice"`
		Currency    string  `json:"currency"`
		CheckoutURL string  `json:"checkoutUrl"`
	}
	json.Unmarshal(post("/api/booking/hold", map[string]any{
		"activityId":        id,
		"date":              date,
		"startTime":         open.StartTime,
		"productOptionCode": code,
		"passengers":        map[string]int{"adults": 2},
		"contact": map[string]string{
			// The traveller's real details. The supplier sends the tickets here.
			"firstName": "<first name>", "lastName": "<last name>",
			"email": "<traveller email>", "phone": "<+ country code and number>",
		},
		"baseUrl": "http://localhost:3000",
	}), &hold)
	fmt.Println("ref:", hold.InternalRef)
	fmt.Println("price:", money(hold.RetailPrice), hold.Currency)

	// 4. Send the customer here. After they pay, call /api/booking/confirm with the session_id.
	fmt.Println("pay at:", hold.CheckoutURL)
}

func main() {
	if key == "" {
		die("Set TOURSCANNER_API_KEY first")
	}
	if len(os.Args) < 2 {
		die("usage: go run . search|availability|book ...")
	}
	switch os.Args[1] {
	case "search":
		search(os.Args[2:])
	case "availability":
		availability(os.Args[2:])
	case "book":
		book(os.Args[2:])
	default:
		die("unknown command %q", os.Args[1])
	}
}
