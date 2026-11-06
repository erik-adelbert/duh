package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func convert(ini, conf string) error {
	in, err := os.Open(ini)
	if err != nil {
		return err
	}
	defer in.Close()

	var (
		section string
		current map[string]string
	)

	sections := make(map[string]map[string]string)

	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = line[1 : len(line)-1]

			var bid int

			if n, err := fmt.Sscanf(section, "bank-%d", &bid); n == 1 && err == nil {
				section = fmt.Sprintf("bank-%02d", bid)
			}

			if _, ok := sections[section]; !ok {
				sections[section] = make(map[string]string)
			}
			current = sections[section]
		case line == "" || strings.HasPrefix(line, ";"):
			// skip empty lines and comments
		default:
			// handle key=value lines
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				current[key] = strings.Trim(value, "\"")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	var cf struct {
		General General                      `json:"general"`
		Banks   map[string]map[string]string `json:"banks"`
	}

	nbank, _ := strconv.Atoi(sections["General"]["banks"])

	x, ok := sections["General"]["auto_id"]
	autoid := ok && x == "true"

	cf.General = General{
		BankCount: uint(nbank),
		AutoID:    autoid,
	}

	delete(sections, "General")

	cf.Banks = sections

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(conf, data, 0644)
}
