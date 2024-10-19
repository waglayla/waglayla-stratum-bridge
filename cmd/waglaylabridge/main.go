package main

import (
	"flag"
	"fmt"
	"github.com/waglayla/waglayla-stratum-bridge/src/waglaylastratum"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v2"
)

func main() {
	pwd, _ := os.Getwd()
	fullPath := path.Join(pwd, "config.yaml")
	log.Printf("loading config @ `%s`", fullPath)
	rawCfg, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Printf("config file not found: %s", err)
		os.Exit(1)
	}
	cfg := waglaylastratum.BridgeConfig{}
	if err := yaml.Unmarshal(rawCfg, &cfg); err != nil {
		log.Printf("failed parsing config file: %s", err)
		os.Exit(1)
	}

	// override flag.Usage for better help output.
	flag.Usage = func() {
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  -%v %v\n", f.Name, f.Value)
			fmt.Fprintf(os.Stderr, "    	%v (default \"%v\")\n", f.Usage, f.Value)
		})
	}

	flag.StringVar(&cfg.StratumPort, "stratum", cfg.StratumPort, "stratum port to listen on, default `:5555`")
	flag.BoolVar(&cfg.PrintStats, "stats", cfg.PrintStats, "true to show periodic stats to console, default `true`")
	flag.StringVar(&cfg.RPCServer, "waglayla", cfg.RPCServer, "address of the waglayla node, default `localhost:13110`")
	flag.DurationVar(&cfg.BlockWaitTime, "blockwait", cfg.BlockWaitTime, "time in ms to wait before manually requesting new block, default `500`")
	flag.Float64Var(&cfg.MinShareDiff, "mindiff", cfg.MinShareDiff, "minimum share difficulty to accept from miner(s), default `4`")
	flag.BoolVar(&cfg.VarDiff, "vardiff", cfg.VarDiff, "true to enable auto-adjusting variable min diff")
	flag.UintVar(&cfg.SharesPerMin, "sharespermin", cfg.SharesPerMin, "number of shares per minute the vardiff engine should target")
	flag.BoolVar(&cfg.VarDiffStats, "vardiffstats", cfg.VarDiffStats, "include vardiff stats readout every 10s in log")
	flag.BoolVar(&cfg.SoloMining, "solo", cfg.SoloMining, "true to use network diff instead of stratum vardiff")
	flag.UintVar(&cfg.ExtranonceSize, "extranonce", cfg.ExtranonceSize, "size in bytes of extranonce, default `0`")
	flag.StringVar(&cfg.PromPort, "prom", cfg.PromPort, "address to serve prom stats, default `:2112`")
	flag.BoolVar(&cfg.UseLogFile, "log", cfg.UseLogFile, "if true will output errors to log file, default `true`")
	flag.StringVar(&cfg.HealthCheckPort, "hcp", cfg.HealthCheckPort, `(rarely used) if defined will expose a health check on /readyz, default ""`)
	flag.Parse()

	if cfg.MinShareDiff == 0 {
		cfg.MinShareDiff = 4
	}
	if cfg.BlockWaitTime == 0 {
		cfg.BlockWaitTime = 5 * time.Second // this should never happen due to waglayla 1s block times
	}

	cfg.SoloMining = true; // temporary for WALA only

	log.Println("----------------------------------")
	log.Printf("initializing bridge")
	log.Printf("\twaglayla:          %s", cfg.RPCServer)
	log.Printf("\tstratum:         %s", cfg.StratumPort)
	log.Printf("\tprom:            %s", cfg.PromPort)
	log.Printf("\tstats:           %t", cfg.PrintStats)
	log.Printf("\tlog:             %t", cfg.UseLogFile)
	log.Printf("\tmin diff:        %.10f", cfg.MinShareDiff)
	log.Printf("\tvar diff:        %t", cfg.VarDiff)
	log.Printf("\tshares per min:  %d", cfg.SharesPerMin)
	log.Printf("\tvar diff stats:  %t", cfg.VarDiffStats)
	log.Printf("\tsolo mining:  	 %t", cfg.SoloMining)
	log.Printf("\tblock wait:      %s", cfg.BlockWaitTime)
	log.Printf("\textranonce size: %d", cfg.ExtranonceSize)
	log.Printf("\thealth check:    %s", cfg.HealthCheckPort)
	log.Println("----------------------------------")

	if err := waglaylastratum.ListenAndServe(cfg); err != nil {
		log.Println(err)
	}
}
