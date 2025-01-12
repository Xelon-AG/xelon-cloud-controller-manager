package xelon

import (
	"slices"

	"github.com/Xelon-AG/xelon-sdk-go/xelon"
)

type ReconcileDiff struct {
	rulesToCreate []xelon.LoadBalancerClusterForwardingRule
	rulesToUpdate []xelon.LoadBalancerClusterForwardingRule
	rulesToDelete []xelon.LoadBalancerClusterForwardingRule
}

func reconcile(currentRules []xelon.LoadBalancerClusterForwardingRule, desiredRules []xelon.LoadBalancerClusterForwardingRule) ReconcileDiff {
	reconcileDiff := ReconcileDiff{}

	// first find rules to create
	for _, desiredRule := range desiredRules {
		found := false
		for _, currentRule := range currentRules {
			if currentRule.Frontend == nil || desiredRule.Frontend == nil {
				continue
			}
			if currentRule.Frontend.Port == desiredRule.Frontend.Port {
				found = true
			}
		}
		if !found {
			reconcileDiff.rulesToCreate = append(reconcileDiff.rulesToCreate, desiredRule)
		}
	}

	// update case: iterate over current rules and find rules with the same frontend port but different backend port
	for _, currentRule := range currentRules {
		for _, desiredRule := range desiredRules {
			if currentRule.Frontend == nil || desiredRule.Frontend == nil {
				continue
			}
			if currentRule.Frontend.Port == desiredRule.Frontend.Port &&
				currentRule.Backend.Port != desiredRule.Backend.Port {
				desiredRule.Frontend.ID = currentRule.Frontend.ID
				desiredRule.Backend.ID = currentRule.Backend.ID
				reconcileDiff.rulesToUpdate = append(reconcileDiff.rulesToUpdate, desiredRule)
				break
			}
		}
	}

	// delete
	for _, currentRule := range currentRules {
		if slices.ContainsFunc(reconcileDiff.rulesToCreate, compareByFrontendPorts(currentRule)) {
			continue
		}
		if slices.ContainsFunc(reconcileDiff.rulesToUpdate, compareByFrontendPorts(currentRule)) {
			continue
		}
		if slices.ContainsFunc(desiredRules, compareByFrontendPorts(currentRule)) {
			continue
		}
		reconcileDiff.rulesToDelete = append(reconcileDiff.rulesToDelete, currentRule)
	}

	return reconcileDiff
}

func compareByFrontendPorts(first xelon.LoadBalancerClusterForwardingRule) func(xelon.LoadBalancerClusterForwardingRule) bool {
	return func(second xelon.LoadBalancerClusterForwardingRule) bool {
		if first.Frontend == nil || second.Frontend == nil {
			return false
		}
		return first.Frontend.Port == second.Frontend.Port
	}
}

func deleteByFrontendID(firstID string) func(string) bool {
	return func(secondID string) bool {
		return firstID == secondID
	}
}
