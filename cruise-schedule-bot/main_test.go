package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseCruises_TodayOnly(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Madrid")
	today := time.Now().In(loc)
	todayStr := today.Format("02/01/2006")
	yesterday := today.AddDate(0, 0, -1).Format("02/01/2006")
	tomorrow := today.AddDate(0, 0, 1).Format("02/01/2006")

	html := `<html><body><table>
		<tr><th>Fecha</th><th>Buque</th><th>Procedencia</th><th>Llegada</th><th>Salida</th><th>Compañía</th></tr>
		<tr><td>` + yesterday + `</td><td>Old Ship</td><td>Lisboa</td><td>08:00</td><td>18:00</td><td>MSC</td></tr>
		<tr><td>` + todayStr + `</td><td>Queen Mary 2</td><td>Southampton</td><td>09:00</td><td>20:00</td><td>Cunard</td></tr>
		<tr><td>` + todayStr + `</td><td>Costa Fascinosa</td><td>Vigo</td><td>10:00</td><td>22:00</td><td>Costa</td></tr>
		<tr><td>` + tomorrow + `</td><td>Future Ship</td><td>Barcelona</td><td>07:00</td><td>19:00</td><td>Royal</td></tr>
	</table></body></html>`

	cruises, err := parseCruises(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cruises) != 2 {
		t.Fatalf("expected 2 cruises for today, got %d", len(cruises))
	}
	if cruises[0].Ship != "Queen Mary 2" {
		t.Errorf("expected Queen Mary 2, got %s", cruises[0].Ship)
	}
	if cruises[0].Departure != "20:00" {
		t.Errorf("expected departure 20:00, got %s", cruises[0].Departure)
	}
	if cruises[1].Ship != "Costa Fascinosa" {
		t.Errorf("expected Costa Fascinosa, got %s", cruises[1].Ship)
	}
}

func TestBuildMessage_Empty(t *testing.T) {
	msg := buildMessage(nil)
	if !strings.Contains(msg, "No hay escalas") {
		t.Errorf("expected no-cruises message, got: %s", msg)
	}
}

func TestBuildMessage_WithCruises(t *testing.T) {
	cruises := []Cruise{
		{Ship: "Queen Mary 2", Origin: "Southampton", Arrival: "09:00", Departure: "20:00", Company: "Cunard"},
	}
	msg := buildMessage(cruises)
	if !strings.Contains(msg, "Queen Mary 2") {
		t.Errorf("ship name not found in message")
	}
	if !strings.Contains(msg, "20:00") {
		t.Errorf("departure time not found in message")
	}
}

func TestEscapeMarkdown(t *testing.T) {
	cases := []struct{ in, want string }{
		{"MSC_Cruises", "MSC\\_Cruises"},
		{"Costa [New]", "Costa \\[New\\]"},
		{"Normal", "Normal"},
	}
	for _, c := range cases {
		got := escapeMarkdown(c.in)
		if got != c.want {
			t.Errorf("escapeMarkdown(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
