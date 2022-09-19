package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type _RateLimitCfgRoot struct {
	AccountingGroups []string                 `json:"groups"`
	Limits           []_RateLimitCfgLimitItem `json:"limits"`
}

type _RateLimitCfgLimitItem struct {
	Seller string `json:"seller"`
	Group  string `json:"group"`
	Value  int    `json:"value"`
}

/*
	Конфигурирует лимитер на основании JSON-файла такого формата:
	{
		"groups":["a", "b"],
		"limits":[
			{"seller": "", "group": "", "value": 20},
			{"seller": "s1", "group": "", "value": 10},
			{"seller": "s1", "group": "a", "value": 15}
		]
	}
*/
func ConfigureRateLimiter(rl IConfigurableRateLimiter, d []byte) error {
	var cfg _RateLimitCfgRoot

	if err := json.Unmarshal(d, &cfg); err != nil {
		return err
	}
	// validate
	tempGroups := map[string]bool{}
	for _, g := range cfg.AccountingGroups {
		if strings.Contains(g, "|") {
			return fmt.Errorf("invalid group name: %s", g)
		}
		if tempGroups[g] {
			return fmt.Errorf("group redeclared: %s", g)
		}
		tempGroups[g] = true
	}
	for _, l := range cfg.Limits {
		if strings.Contains(l.Seller, "|") {
			return fmt.Errorf("invalid seller id: %s", l.Seller)
		}
		if l.Group != "" {
			if !tempGroups[l.Group] {
				return fmt.Errorf("group does not exist: %s", l.Group)
			}
		}
		if l.Value < 0 && l.Value != Unlimited {
			return fmt.Errorf("invalid limit: %d", l.Value)
		}
	}

	rl.SetAccountingGroups(cfg.AccountingGroups)
	for _, x := range cfg.Limits {
		if err := rl.SetLimit(x.Seller, x.Group, x.Value); err != nil {
			return err
		}
	}
	return nil
}
