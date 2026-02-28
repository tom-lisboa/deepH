package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"deeph/internal/commanddoc"
)

func cmdCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("command requires a subcommand: list or explain")
	}
	switch args[0] {
	case "list":
		return cmdCommandList(args[1:])
	case "explain":
		return cmdCommandExplain(args[1:])
	default:
		return fmt.Errorf("unknown command subcommand %q", args[0])
	}
}

func cmdCommandList(args []string) error {
	fs := flag.NewFlagSet("command list", flag.ContinueOnError)
	category := fs.String("category", "", "filter by category (workspace, execution, providers, ...)")
	jsonOut := fs.Bool("json", false, "print command dictionary as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	filter := strings.TrimSpace(strings.ToLower(*category))

	docs := commanddoc.Dictionary()
	grouped := map[string][]commanddoc.Doc{}
	cats := make([]string, 0, 8)
	for _, d := range docs {
		cat := strings.TrimSpace(strings.ToLower(d.Category))
		if filter != "" && cat != filter {
			continue
		}
		if _, ok := grouped[cat]; !ok {
			cats = append(cats, cat)
		}
		grouped[cat] = append(grouped[cat], d)
	}
	if len(cats) == 0 {
		if filter == "" {
			fmt.Println("No commands registered.")
			return nil
		}
		return fmt.Errorf("no commands found for category %q", filter)
	}
	if *jsonOut {
		type commandListPayload struct {
			Category string           `json:"category,omitempty"`
			Commands []commanddoc.Doc `json:"commands"`
		}
		payload := commandListPayload{Category: filter}
		for _, cat := range cats {
			items := grouped[cat]
			sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
			payload.Commands = append(payload.Commands, items...)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	sort.Strings(cats)
	for i, cat := range cats {
		if i > 0 {
			fmt.Println("")
		}
		fmt.Printf("[%s]\n", cat)
		items := grouped[cat]
		sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
		for _, d := range items {
			fmt.Printf("- %s: %s\n", d.Path, d.Summary)
		}
	}
	return nil
}

func cmdCommandExplain(args []string) error {
	fs := flag.NewFlagSet("command explain", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print command entry as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New(`command explain requires "<command path>" (ex.: "provider add")`)
	}
	path := commanddoc.NormalizePath(strings.Join(rest, " "))
	doc, ok := commanddoc.Lookup(path)
	if !ok {
		return fmt.Errorf("unknown command path %q (tip: use `deeph command list`)", path)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	fmt.Printf("command: %s\n", doc.Path)
	fmt.Printf("category: %s\n", doc.Category)
	fmt.Printf("summary: %s\n", doc.Summary)
	if len(doc.Usage) > 0 {
		fmt.Println("usage:")
		for _, u := range doc.Usage {
			fmt.Printf("  - %s\n", u)
		}
	}
	if len(doc.Examples) > 0 {
		fmt.Println("examples:")
		for _, ex := range doc.Examples {
			fmt.Printf("  - %s\n", ex)
		}
	}
	if len(doc.Notes) > 0 {
		fmt.Println("notes:")
		for _, n := range doc.Notes {
			fmt.Printf("  - %s\n", n)
		}
	}
	return nil
}
