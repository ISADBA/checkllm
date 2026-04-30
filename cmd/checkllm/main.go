package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
	"github.com/ISADBA/checkllm/internal/config"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	result, err := runcheck.NewService().Run(context.Background(), runcheck.Input{
		BaseURL:      cfg.BaseURL,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		Provider:     cfg.Provider,
		BaselinePath: cfg.BaselinePath,
		Timeout:      cfg.Timeout,
		MaxSamples:   cfg.MaxSamples,
		EnableStream: cfg.EnableStream,
		ExpectUsage:  cfg.ExpectUsage,
		OutputPath:   cfg.OutputPath,
		WriteReport:  true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run archive written to %s\n", result.ArchivePath)
	fmt.Printf("user report written to %s\n", result.UserReportPath)
	fmt.Printf("protocol=%d usage=%d fingerprint=%d tier=%d route=%d risk=%d conclusion=%s\n",
		int(result.Summary.Scores.Protocol),
		int(result.Summary.Scores.Usage),
		int(result.Summary.Scores.Fingerprint),
		int(result.Summary.Scores.Tier),
		int(result.Summary.Scores.Route),
		int(result.Summary.Scores.Risk),
		result.Summary.Conclusion,
	)
}
