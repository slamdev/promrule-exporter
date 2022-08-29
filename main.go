package main

import (
	"context"
	"flag"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"golang.org/x/exp/maps"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

func main() {
	var excludeAlertRules bool
	flag.BoolVar(&excludeAlertRules, "exclude-alert-rules", false, "Exclude alert rules from PrometheusRule resources")

	var excludeRecordingRules bool
	flag.BoolVar(&excludeRecordingRules, "exclude-recording-rules", false, "Exclude recording rules from PrometheusRule resources")

	var outputDir string
	flag.StringVar(&outputDir, "output-dir", "", "Directory to save extracted rules")

	flag.Parse()

	ctx := context.Background()
	c, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		klog.Fatal(err)
	}
	if err := monitoring.AddToScheme(c.Scheme()); err != nil {
		klog.Fatal(err)
	}

	ruleList := monitoring.PrometheusRuleList{}

	if err := c.List(ctx, &ruleList, &client.ListOptions{}); err != nil {
		klog.Fatal(err)
	}

	klog.Infof("found [%d] PrometheusRule resources in all namespaces", len(ruleList.Items))

	groupsByNamespace := map[string]map[string]monitoring.RuleGroup{}

	for _, rule := range ruleList.Items {
		groups := rule.Spec.Groups
		if existingGroupsByNamespace, ok := groupsByNamespace[rule.Namespace]; ok {
			for _, group := range groups {
				klog.Infof("processing [%s] group", group.Name)

				filteredRules := filterRules(group.Rules, excludeAlertRules, excludeRecordingRules, rule.Namespace, group.Name)
				group.Rules = filteredRules
				if len(group.Rules) == 0 {
					klog.Infof("no rules are left in [%s] group after filtering; skipping", group.Name)
					continue
				}

				if existingGroup, ok := existingGroupsByNamespace[group.Name]; ok {
					existingGroup.Rules = append(existingGroup.Rules, group.Rules...)
				} else {
					existingGroupsByNamespace[group.Name] = group
				}
				klog.Infof("[%s] group with [%d] rules is added to [%s] namespace", group.Name, len(group.Rules), rule.Namespace)
			}
		} else {
			groupsByName := map[string]monitoring.RuleGroup{}
			for _, group := range groups {
				klog.Infof("processing [%s] group", group.Name)

				filteredRules := filterRules(group.Rules, excludeAlertRules, excludeRecordingRules, rule.Namespace, group.Name)
				group.Rules = filteredRules
				if len(group.Rules) == 0 {
					klog.Infof("no rules are left in [%s] group after filtering; skipping", group.Name)
					continue
				}

				groupsByName[group.Name] = group
				klog.Infof("[%s] group with [%d] rules is added to [%s] namespace", group.Name, len(group.Rules), rule.Namespace)
			}

			if len(groupsByName) != 0 {
				groupsByNamespace[rule.Namespace] = groupsByName
			}
		}
	}

	for namespace, groupsByName := range groupsByNamespace {
		groups := maps.Values(groupsByName)
		res := map[string]interface{}{
			"namespace": namespace,
			"groups":    groups,
		}
		out, err := yaml.Marshal(res)
		if err != nil {
			klog.Fatal(err)
		}
		outFile := filepath.Join(outputDir, namespace+".yaml")
		if err := os.WriteFile(outFile, out, 0644); err != nil {
			klog.Fatal(err)
		}
		klog.Infof("[%d] groups are written to [%s] file", len(groups), outFile)
	}
}

func filterRules(rules []monitoring.Rule, excludeAlertRules bool, excludeRecordingRules bool, namespace string, group string) []monitoring.Rule {
	if !excludeAlertRules && !excludeRecordingRules {
		return rules
	}
	var res []monitoring.Rule
	for _, rule := range rules {
		if excludeAlertRules && rule.Alert != "" {
			continue
		}
		if excludeRecordingRules && rule.Record != "" {
			continue
		}
		rule.Labels["rule-namespace"] = namespace
		rule.Labels["rule-group"] = group
		res = append(res, rule)
	}
	return res
}
