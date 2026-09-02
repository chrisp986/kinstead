package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"game/backend/internal/simulation"
)

func main() {
	csvPath := flag.String("csv", "", "write summary CSV to this path")
	flag.Parse()

	summaries, err := simulation.RunAllV03Scenarios()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%-12s %10s %8s %8s %8s %8s %10s %6s  %s\n", "strategy", "supply_d", "silver", "wood", "strained", "critical", "trade", "jarl", "buildings")
	for _, s := range summaries {
		fmt.Printf("%-12s %10.1f %8.1f %8.1f %8d %8d %10.1f %6d  %s\n",
			s.Strategy, s.SupplyDays, s.Silver, s.Wood, s.StrainedDays, s.CriticalDays, s.TradeVolume, s.JarlStanding, strings.Join(s.Buildings, ","))
	}

	if *csvPath != "" {
		f, err := os.Create(*csvPath)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		defer w.Flush()
		_ = w.Write([]string{"strategy", "supply_days", "silver", "wood", "strained_days", "critical_days", "trade_volume", "jarl_standing", "political_service_days", "political_wood_paid", "political_silver_paid", "buildings"})
		for _, s := range summaries {
			_ = w.Write([]string{string(s.Strategy), fmt.Sprintf("%.2f", s.SupplyDays), fmt.Sprintf("%.2f", s.Silver), fmt.Sprintf("%.2f", s.Wood), strconv.Itoa(s.StrainedDays), strconv.Itoa(s.CriticalDays), fmt.Sprintf("%.2f", s.TradeVolume), strconv.Itoa(s.JarlStanding), strconv.Itoa(s.PoliticalServiceDays), fmt.Sprintf("%.2f", s.PoliticalWoodPaid), fmt.Sprintf("%.2f", s.PoliticalSilverPaid), strings.Join(s.Buildings, "|")})
		}
		if err := w.Error(); err != nil {
			panic(err)
		}
		fmt.Println("wrote", *csvPath)
	}
}
