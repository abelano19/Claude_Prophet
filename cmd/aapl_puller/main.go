package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"prophet-trader/config"
	"prophet-trader/interfaces"
	"prophet-trader/services"
)

func main() {
	// Load config from .env; fall back to raw env vars if file missing.
	if err := config.Load(); err != nil {
		log.Printf("Warning: could not load .env file (%v) — using environment variables", err)
		config.AppConfig = &config.Config{
			AlpacaAPIKey:    os.Getenv("ALPACA_API_KEY"),
			AlpacaSecretKey: os.Getenv("ALPACA_SECRET_KEY"),
		}
	}

	if config.AppConfig.AlpacaAPIKey == "" || config.AppConfig.AlpacaSecretKey == "" {
		log.Fatal("ALPACA_API_KEY and ALPACA_SECRET_KEY must be set in .env or environment")
	}

	ctx := context.Background()
	const symbol = "AAPL"

	fmt.Println("=================================================================")
	fmt.Printf("  AAPL DATA PULLER  —  %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Println("=================================================================")

	dataSvc := services.NewAlpacaDataService(config.AppConfig.AlpacaAPIKey, config.AppConfig.AlpacaSecretKey)
	optSvc := services.NewAlpacaOptionsDataService(config.AppConfig.AlpacaAPIKey, config.AppConfig.AlpacaSecretKey)

	pullEquityData(ctx, dataSvc, symbol)
	pullOptionsData(ctx, optSvc, symbol)
}

// ─── Equity ───────────────────────────────────────────────────────────────────

func pullEquityData(ctx context.Context, svc *services.AlpacaDataService, symbol string) {
	header("EQUITY DATA — " + symbol)

	// Real-time quote
	section("Latest Quote (real-time bid/ask)")
	quote, err := svc.GetLatestQuote(ctx, symbol)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printQuote(quote)
	}

	// Real-time last trade
	section("Latest Trade (real-time last print)")
	trade, err := svc.GetLatestTrade(ctx, symbol)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printTrade(trade)
	}

	// Most recent candle
	section("Latest Bar (most recent candle)")
	bar, err := svc.GetLatestBar(ctx, symbol)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printBar(bar)
	}

	// Historical daily bars — last 30 calendar days
	section("Historical Daily Bars (last 30 days)")
	end := time.Now()
	start := end.AddDate(0, 0, -30)
	bars, err := svc.GetHistoricalBars(ctx, symbol, start, end, "1Day")
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printBarsTable(bars)
	}
}

func printQuote(q *interfaces.Quote) {
	fmt.Printf("  Bid:    $%-10.4f  (size %d)\n", q.BidPrice, q.BidSize)
	fmt.Printf("  Ask:    $%-10.4f  (size %d)\n", q.AskPrice, q.AskSize)
	fmt.Printf("  Spread: $%.4f\n", q.AskPrice-q.BidPrice)
	fmt.Printf("  Time:    %s\n", q.Timestamp.Local().Format("2006-01-02 15:04:05"))
}

func printTrade(t *interfaces.Trade) {
	fmt.Printf("  Price:  $%.4f\n", t.Price)
	fmt.Printf("  Size:   %d shares\n", t.Size)
	fmt.Printf("  Time:   %s\n", t.Timestamp.Local().Format("2006-01-02 15:04:05"))
}

func printBar(b *interfaces.Bar) {
	fmt.Printf("  Open:   $%.4f\n", b.Open)
	fmt.Printf("  High:   $%.4f\n", b.High)
	fmt.Printf("  Low:    $%.4f\n", b.Low)
	fmt.Printf("  Close:  $%.4f\n", b.Close)
	fmt.Printf("  Volume: %d\n", b.Volume)
	fmt.Printf("  VWAP:   $%.4f\n", b.VWAP)
	fmt.Printf("  Time:   %s\n", b.Timestamp.Local().Format("2006-01-02 15:04:05"))
}

func printBarsTable(bars []*interfaces.Bar) {
	fmt.Printf("  %-12s  %8s  %8s  %8s  %8s  %12s  %8s\n",
		"Date", "Open", "High", "Low", "Close", "Volume", "VWAP")
	fmt.Println("  " + repeat("─", 72))
	for _, b := range bars {
		fmt.Printf("  %-12s  %8.2f  %8.2f  %8.2f  %8.2f  %12d  %8.2f\n",
			b.Timestamp.Format("2006-01-02"),
			b.Open, b.High, b.Low, b.Close, b.Volume, b.VWAP)
	}
	fmt.Printf("  Total: %d bars\n", len(bars))
}

// ─── Options ──────────────────────────────────────────────────────────────────

func pullOptionsData(ctx context.Context, svc *services.AlpacaOptionsDataService, symbol string) {
	header("OPTIONS DATA — " + symbol)

	// Contracts near 7 DTE (weekly / short-dated scalps)
	section("Contracts Near 7 DTE — calls (±3 day tolerance)")
	c7, err := svc.FindOptionsNearDTE(ctx, symbol, 7, 3)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printContractsTable(c7)
	}

	// Contracts near 30 DTE (standard monthly swing)
	section("Contracts Near 30 DTE — calls (±7 day tolerance)")
	c30, err := svc.FindOptionsNearDTE(ctx, symbol, 30, 7)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printContractsTable(c30)
	}

	// Contracts near 90 DTE (LEAPS / longer-dated)
	section("Contracts Near 90 DTE — calls (±14 day tolerance)")
	c90, err := svc.FindOptionsNearDTE(ctx, symbol, 90, 14)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printContractsTable(c90)
	}

	// Full chain for the nearest standard monthly expiration
	nextExp := nextMonthlyExpiration()
	section(fmt.Sprintf("Full Options Chain — nearest monthly expiry %s", nextExp.Format("2006-01-02")))
	chain, err := svc.GetOptionChain(ctx, symbol, nextExp)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		printSplitChain(chain)
	}
}

func printContractsTable(contracts map[string]*interfaces.OptionContract) {
	if len(contracts) == 0 {
		fmt.Println("  (no contracts returned)")
		return
	}

	// Sort by strike price for readability
	keys := make([]string, 0, len(contracts))
	for k := range contracts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return contracts[keys[i]].StrikePrice < contracts[keys[j]].StrikePrice
	})

	fmt.Printf("  %-26s  %4s  %8s  %5s  %8s  %10s\n",
		"Symbol", "DTE", "Strike", "Type", "Expiry", "Open Int")
	fmt.Println("  " + repeat("─", 70))
	for _, k := range keys {
		c := contracts[k]
		fmt.Printf("  %-26s  %4d  %8.2f  %5s  %8s  %10d\n",
			c.Symbol, c.DTE, c.StrikePrice, c.ContractType,
			c.ExpirationDate.Format("01/02/06"), c.OpenInterest)
	}
	fmt.Printf("  Total: %d contracts\n", len(contracts))
}

func printSplitChain(chain map[string]*interfaces.OptionContract) {
	var calls, puts []*interfaces.OptionContract
	for _, c := range chain {
		if c.ContractType == "call" {
			calls = append(calls, c)
		} else {
			puts = append(puts, c)
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].StrikePrice < calls[j].StrikePrice })
	sort.Slice(puts, func(i, j int) bool { return puts[i].StrikePrice < puts[j].StrikePrice })

	fmt.Printf("  Total contracts: %d  (calls: %d, puts: %d)\n", len(chain), len(calls), len(puts))
	printChainSection("CALLS", calls, 20)
	printChainSection("PUTS", puts, 20)
}

func printChainSection(label string, contracts []*interfaces.OptionContract, maxRows int) {
	fmt.Printf("\n  ── %s (%d total) ──\n", label, len(contracts))
	fmt.Printf("  %-26s  %8s  %5s  %10s\n", "Symbol", "Strike", "DTE", "Open Int")
	fmt.Println("  " + repeat("─", 55))
	limit := maxRows
	if len(contracts) < limit {
		limit = len(contracts)
	}
	for _, c := range contracts[:limit] {
		fmt.Printf("  %-26s  %8.2f  %5d  %10d\n",
			c.Symbol, c.StrikePrice, c.DTE, c.OpenInterest)
	}
	if len(contracts) > maxRows {
		fmt.Printf("  ... and %d more\n", len(contracts)-maxRows)
	}
}

// ─── Date helpers ─────────────────────────────────────────────────────────────

// nextMonthlyExpiration returns the nearest 3rd-Friday monthly expiration
// that is at least 5 days away.
func nextMonthlyExpiration() time.Time {
	now := time.Now()
	for monthOffset := 0; monthOffset <= 3; monthOffset++ {
		t := time.Date(now.Year(), now.Month()+time.Month(monthOffset), 1, 0, 0, 0, 0, time.UTC)
		fri := thirdFriday(t.Year(), t.Month())
		if fri.Sub(now) > 5*24*time.Hour {
			return fri
		}
	}
	// Fallback: 2 months out
	t := time.Date(now.Year(), now.Month()+2, 1, 0, 0, 0, 0, time.UTC)
	return thirdFriday(t.Year(), t.Month())
}

func thirdFriday(year int, month time.Month) time.Time {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(time.Friday) - int(first.Weekday()) + 7) % 7
	return first.AddDate(0, 0, offset+14)
}

// ─── Display helpers ──────────────────────────────────────────────────────────

func header(title string) {
	line := repeat("═", len(title)+4)
	fmt.Printf("\n╔%s╗\n║  %s  ║\n╚%s╝\n", line, title, line)
}

func section(title string) {
	fmt.Printf("\n▶ %s\n", title)
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
