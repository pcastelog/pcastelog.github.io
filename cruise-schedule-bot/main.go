package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	scheduleURL = "https://www.puertocoruna.com/escalas-de-cruceros"
	telegramAPI = "https://api.telegram.org/bot%s/sendMessage"
)

type Event struct{}

type Cruise struct {
	Ship      string
	Origin    string
	Arrival   string
	Departure string
	Company   string
}

func handler(ctx context.Context, _ Event) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID env vars are required")
	}

	cruises, err := fetchTodayCruises()
	if err != nil {
		return fmt.Errorf("fetching cruises: %w", err)
	}

	message := buildMessage(cruises)
	return sendTelegram(botToken, chatID, message)
}

func fetchTodayCruises() ([]Cruise, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", scheduleURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return parseCruises(resp.Body)
}

func parseCruises(r io.Reader) ([]Cruise, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	// Use Madrid timezone (Spain)
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc)

	// Try multiple date formats used by Spanish port websites
	dateFormats := []string{
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2006-01-02",
	}
	todayStr := today.Format("02/01/2006")

	var cruises []Cruise

	// The schedule table typically has columns:
	// Fecha | Buque | Procedencia | Llegada | Salida | Compañía
	doc.Find("table tr").Each(func(i int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 5 {
			return
		}

		rawDate := strings.TrimSpace(cells.Eq(0).Text())

		matched := false
		for _, fmt := range dateFormats {
			parsed, err := time.ParseInLocation(fmt, rawDate, loc)
			if err == nil {
				if parsed.Year() == today.Year() &&
					parsed.Month() == today.Month() &&
					parsed.Day() == today.Day() {
					matched = true
					break
				}
				// If year is zero (e.g. "02/01" short format) check month+day
				if parsed.Year() == 0 &&
					parsed.Month() == today.Month() &&
					parsed.Day() == today.Day() {
					matched = true
					break
				}
			}
		}

		// Fallback: plain string comparison
		if !matched {
			normalized := strings.ReplaceAll(rawDate, "-", "/")
			if normalized == todayStr {
				matched = true
			}
		}

		if !matched {
			return
		}

		cruise := Cruise{
			Ship:      strings.TrimSpace(cells.Eq(1).Text()),
			Origin:    strings.TrimSpace(cells.Eq(2).Text()),
			Arrival:   strings.TrimSpace(cells.Eq(3).Text()),
			Departure: strings.TrimSpace(cells.Eq(4).Text()),
		}
		if cells.Length() >= 6 {
			cruise.Company = strings.TrimSpace(cells.Eq(5).Text())
		}

		if cruise.Ship != "" {
			cruises = append(cruises, cruise)
		}
	})

	return cruises, nil
}

func buildMessage(cruises []Cruise) string {
	loc, _ := time.LoadLocation("Europe/Madrid")
	today := time.Now().In(loc)
	dateStr := today.Format("02/01/2006")

	if len(cruises) == 0 {
		return fmt.Sprintf("🚢 Cruceros en A Coruña — %s\n\nNo hay escalas programadas para hoy.", dateStr)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚢 Cruceros en A Coruña — %s\n\n", dateStr))

	for _, c := range cruises {
		sb.WriteString(fmt.Sprintf("*%s*\n", escapeMarkdown(c.Ship)))
		if c.Origin != "" {
			sb.WriteString(fmt.Sprintf("  Procedencia: %s\n", c.Origin))
		}
		if c.Arrival != "" {
			sb.WriteString(fmt.Sprintf("  Llegada:   %s\n", c.Arrival))
		}
		if c.Departure != "" {
			sb.WriteString(fmt.Sprintf("  Salida:    %s\n", c.Departure))
		}
		if c.Company != "" {
			sb.WriteString(fmt.Sprintf("  Compañía:  %s\n", c.Company))
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// escapeMarkdown escapes Telegram MarkdownV1 special characters.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
	)
	return replacer.Replace(s)
}

func sendTelegram(botToken, chatID, text string) error {
	apiURL := fmt.Sprintf(telegramAPI, botToken)

	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	params.Set("parse_mode", "Markdown")

	resp, err := http.PostForm(apiURL, params)
	if err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing telegram response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram API error: %s", result.Description)
	}

	log.Printf("Telegram message sent to chat %s (%d cruises)", chatID, strings.Count(text, "\n*"))
	return nil
}

func main() {
	lambda.Start(handler)
}
