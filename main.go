package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type Result struct {
	InstanceID    string
	IP            string
	LaunchTime    time.Time
	ResponseTime  time.Duration
	Error         error
	InstanceState string
}

type Config struct {
	ASGName     string
	Region      string
	Path        string
	Port        string
	TLSEnabled  bool
	PostFile    string
	RequestType string
	Timeout     time.Duration
	Headers     map[string]string
}

func parseFlags(fs *flag.FlagSet, args []string) (*Config, error) {
	cfg := &Config{}
	var headersRaw string
	fs.StringVar(&cfg.ASGName, "asg-name", "", "Name of the Auto Scaling Group (required)")
	fs.StringVar(&cfg.Region, "region", "", "AWS region (required)")
	fs.StringVar(&cfg.Path, "path", "/", "HTTP path to call on each instance (default: /)")
	fs.StringVar(&cfg.Port, "port", "80", "Port to use for the HTTP request (default: 80)")
	fs.BoolVar(&cfg.TLSEnabled, "tls", false, "Enable TLS (use HTTPS instead of HTTP) (default: false)")
	fs.StringVar(&cfg.PostFile, "post", "", "File to POST as request body (if set, POST is used instead of GET) (eg: some-request.json)")
	fs.StringVar(&cfg.RequestType, "request-type", "application/json", "Content-Type for the request (default: application/json)")
	fs.DurationVar(&cfg.Timeout, "timeout", 3*time.Second, "HTTP request timeout (default: 3s, example: 1.5s, 500ms, 2m)")
	fs.StringVar(&headersRaw, "headers", "", "Comma-separated list of headers (key=value,key2=value2)")
	err := fs.Parse(args)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	if cfg.ASGName == "" || cfg.Region == "" {
		return nil, fmt.Errorf("asg-name and region are required")
	}

	if cfg.Path == "" || cfg.Path[0] != '/' {
		cfg.Path = "/" + cfg.Path
	}

	if cfg.PostFile != "" {
		if _, err := os.Stat(cfg.PostFile); os.IsNotExist(err) {
			return nil, fmt.Errorf("POST file does not exist: %s", cfg.PostFile)
		}
	}

	cfg.Headers = make(map[string]string)
	if headersRaw != "" {
		headerPairs := splitAndTrim(headersRaw, ",")
		for _, pair := range headerPairs {
			kv := splitAndTrim(pair, "=")
			if len(kv) == 2 {
				cfg.Headers[kv[0]] = kv[1]
			}
		}
	}

	return cfg, nil
}

// splitAndTrim splits a string by sep and trims spaces from each part
func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, p := range bytes.Split([]byte(s), []byte(sep)) {
		parts = append(parts, string(bytes.TrimSpace(p)))
	}
	return parts
}

func main() {
	cfg, err := parseFlags(flag.CommandLine, os.Args[1:])
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	instanceIDs, err := getASGInstanceIDs(awsCfg, cfg.ASGName)
	if err != nil {
		log.Fatalf("Error getting instances: %v", err)
	}

	instances, err := getInstanceMetadata(awsCfg, instanceIDs)
	if err != nil {
		log.Fatalf("Error retrieving instance metadata: %v", err)
	}

	results := makeRequests(cfg, instances)
	printResults(results)
}

func makeRequests(cfg *Config, instances []Result) []Result {
	var wg sync.WaitGroup
	resultsChan := make(chan Result, len(instances))

	for i := range instances {
		inst := instances[i]
		if inst.InstanceState != "running" {
			inst.Error = nil
			inst.ResponseTime = 0
			resultsChan <- Result{
				InstanceID:    inst.InstanceID,
				IP:            inst.IP,
				LaunchTime:    inst.LaunchTime,
				ResponseTime:  0,
				Error:         nil,
				InstanceState: inst.InstanceState,
			}
			continue
		}
		wg.Add(1)
		go func(inst Result) {
			defer wg.Done()
			start := time.Now()
			client := http.Client{Timeout: cfg.Timeout}
			protocol := "http"
			if cfg.TLSEnabled {
				protocol = "https"
			}
			url := fmt.Sprintf("%s://%s:%s%s", protocol, inst.IP, cfg.Port, cfg.Path)

			var resp *http.Response
			if cfg.PostFile != "" {
				data, err := os.ReadFile(cfg.PostFile)
				if err != nil {
					inst.Error = fmt.Errorf("failed to read POST file: %w", err)
					resultsChan <- inst
					return
				}
				req, err := http.NewRequest("POST", url, bytes.NewReader(data))
				if err != nil {
					inst.Error = err
					resultsChan <- inst
					return
				}
				req.Header.Set("Content-Type", cfg.RequestType)
				for k, v := range cfg.Headers {
					req.Header.Set(k, v)
				}
				resp, err = client.Do(req)
				if err != nil {
					inst.Error = err
					resultsChan <- inst
					return
				}
			} else {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					inst.Error = err
					resultsChan <- inst
					return
				}
				for k, v := range cfg.Headers {
					req.Header.Set(k, v)
				}
				resp, err = client.Do(req)
				if err != nil {
					inst.Error = err
					resultsChan <- inst
					return
				}
			}
			defer resp.Body.Close()
			_, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				inst.Error = fmt.Errorf("failed to read response body: %w", err)
				resultsChan <- inst
				return
			}
			inst.ResponseTime = time.Since(start)
			inst.Error = nil
			resultsChan <- inst
		}(inst)
	}
	wg.Wait()
	close(resultsChan)

	var results []Result
	for inst := range resultsChan {
		results = append(results, inst)
	}
	return results
}

func printResults(results []Result) {
	fmt.Printf("\n%-20s %-15s %-25s %-12s %-15s %s\n", "Instance ID", "IP", "Launch Time", "State", "Resp Time", "Status")
	for _, inst := range results {
		status := "OK"
		if inst.InstanceState != "running" {
			status = "Skipped"
		} else if inst.Error != nil {
			status = inst.Error.Error()
		}
		fmt.Printf("%-20s %-15s %-25s %-12s %-15s %s\n",
			inst.InstanceID,
			inst.IP,
			inst.LaunchTime.Format(time.RFC3339),
			inst.InstanceState,
			inst.ResponseTime,
			status,
		)
	}
}

func getASGInstanceIDs(cfg aws.Config, asgName string) ([]string, error) {
	client := autoscaling.NewFromConfig(cfg)
	resp, err := client.DescribeAutoScalingGroups(context.Background(), &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("no ASG found with name %s", asgName)
	}

	var ids []string
	for _, inst := range resp.AutoScalingGroups[0].Instances {
		ids = append(ids, *inst.InstanceId)
	}
	return ids, nil
}

func getInstanceMetadata(cfg aws.Config, instanceIDs []string) ([]Result, error) {
	client := ec2.NewFromConfig(cfg)
	resp, err := client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, r := range resp.Reservations {
		for _, inst := range r.Instances {
			if inst.PrivateIpAddress != nil {
				state := "unknown"
				if inst.State != nil && inst.State.Name != "" {
					state = string(inst.State.Name)
				}
				results = append(results, Result{
					InstanceID:    *inst.InstanceId,
					IP:            *inst.PrivateIpAddress,
					LaunchTime:    *inst.LaunchTime,
					InstanceState: state,
				})
			}
		}
	}
	return results, nil
}
