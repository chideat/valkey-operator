/*
Copyright 2024 chideat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"fmt"
	"slices"
	"strings"
)

var (
	allowedCategories = []string{
		"keyspace", "read", "write", "set", "sortedset", "list", "hash", "string",
		"bitmap", "hyperloglog", "geo", "stream", "pubsub", "admin", "fast", "slow",
		"blocking", "dangerous", "connection", "transaction", "scripting", "all",
	}
)

// Rule acl rules
type Rule struct {
	// Categories
	Categories []string `json:"categories,omitempty"`
	// DisallowedCategories
	DisallowedCategories []string `json:"disallowedCategories,omitempty"`
	// AllowedCommands supports <command> and <command>|<subcommand>
	AllowedCommands []string `json:"allowedCommands,omitempty"`
	// DisallowedCommands supports <command> and <command>|<subcommand>
	DisallowedCommands []string `json:"disallowedCommands,omitempty"`
	// KeyPatterns support multi patterns
	KeyPatterns []string `json:"keyPatterns,omitempty"`
	// KeyReadPatterns >= 7.0 support key read patterns
	KeyReadPatterns []string `json:"keyReadPatterns,omitempty"`
	// KeyWritePatterns >= 7.0 support key write patterns
	KeyWritePatterns []string `json:"keyWritePatterns,omitempty"`
	// Channels >= 7.0 support channel patterns
	Channels []string `json:"channels,omitempty"`
}

// NewRule
func NewRule(val string) (*Rule, error) {
	if val == "" {
		return &Rule{}, nil
	}

	r := Rule{}
	for _, v := range strings.Fields(val) {
		if strings.ToLower(v) == "allcommands" {
			if !slices.Contains(r.Categories, "all") {
				r.Categories = append([]string{"all"}, r.Categories...)
			}
		} else if strings.ToLower(v) == "nocommands" {
			if !slices.Contains(r.DisallowedCategories, "all") {
				r.DisallowedCategories = append([]string{"all"}, r.DisallowedCategories...)
			}
		} else if strings.HasPrefix(v, "+@") {
			v = strings.TrimPrefix(v, "+@")
			if !slices.Contains(r.Categories, v) {
				r.Categories = append(r.Categories, v)
			}
		} else if strings.HasPrefix(v, "-@") {
			v = strings.TrimPrefix(v, "-@")
			if !slices.Contains(r.DisallowedCategories, v) {
				r.DisallowedCategories = append(r.DisallowedCategories, v)
			}
		} else if strings.HasPrefix(v, "-") {
			v = strings.TrimPrefix(v, "-")
			if !slices.Contains(r.DisallowedCommands, v) {
				r.DisallowedCommands = append(r.DisallowedCommands, v)
			}
		} else if strings.HasPrefix(v, "+") {
			v = strings.TrimPrefix(v, "+")
			if !slices.Contains(r.AllowedCommands, v) {
				r.AllowedCommands = append(r.AllowedCommands, v)
			}
		} else if strings.ToLower(v) == "allkeys" {
			if !slices.Contains(r.KeyPatterns, "*") {
				r.KeyPatterns = append([]string{"*"}, r.KeyPatterns...)
			}
		} else if strings.HasPrefix(v, "~") {
			v = strings.TrimPrefix(v, "~")
			if !slices.Contains(r.KeyPatterns, v) {
				r.KeyPatterns = append(r.KeyPatterns, v)
			}
		} else if strings.HasPrefix(v, "%R~") {
			v = strings.TrimPrefix(v, "%R~")
			if !slices.Contains(r.KeyReadPatterns, v) {
				r.KeyReadPatterns = append(r.KeyReadPatterns, v)
			}
		} else if strings.HasPrefix(v, "%W~") {
			v = strings.TrimPrefix(v, "%W~")
			if !slices.Contains(r.KeyWritePatterns, v) {
				r.KeyWritePatterns = append(r.KeyWritePatterns, v)
			}
		} else if strings.ToLower(v) == "allchannels" {
			if !slices.Contains(r.Channels, "*") {
				r.Channels = append([]string{"*"}, r.Channels...)
			}
		} else if strings.HasPrefix(v, "&") {
			v = strings.TrimPrefix(v, "&")
			if !slices.Contains(r.Channels, v) {
				r.Channels = append(r.Channels, v)
			}
		} else {
			return nil, fmt.Errorf("unsupported rule %s", v)
		}
	}
	for _, cate := range append(append([]string{}, r.Categories...), r.DisallowedCategories...) {
		if !slices.Contains(allowedCategories, cate) {
			return nil, fmt.Errorf("unsupported category %s", cate)
		}
	}
	return &r, nil
}

func (rule *Rule) Encode() string {
	var (
		args           []string
		enabledAllCmd  bool
		disabledAllCmd bool
	)
	for _, cate := range rule.Categories {
		if cate == "all" {
			enabledAllCmd = true
			continue
		}
		args = append(args, fmt.Sprintf("+@%s", cate))
	}
	for _, cate := range rule.DisallowedCategories {
		if cate == "all" {
			disabledAllCmd = true
			continue
		}
		args = append(args, fmt.Sprintf("-@%s", cate))
	}
	var subCmds []string
	for _, cmd := range rule.AllowedCommands {
		if strings.Contains(cmd, "|") {
			subCmds = append(subCmds, fmt.Sprintf("+%s", cmd))
		} else {
			args = append(args, fmt.Sprintf("+%s", cmd))
		}
	}
	for _, cmd := range rule.DisallowedCommands {
		args = append(args, fmt.Sprintf("-%s", cmd))
	}
	args = append(args, subCmds...)

	if disabledAllCmd {
		args = append([]string{"-@all"}, args...)
	}
	if enabledAllCmd {
		args = append([]string{"+@all"}, args...)
	}
	for _, pattern := range rule.KeyPatterns {
		args = append(args, fmt.Sprintf("~%s", pattern))
	}
	for _, pattern := range rule.KeyReadPatterns {
		args = append(args, fmt.Sprintf("%%R~%s", pattern))
	}
	for _, pattern := range rule.KeyWritePatterns {
		args = append(args, fmt.Sprintf("%%W~%s", pattern))
	}
	for _, pattern := range rule.Channels {
		args = append(args, fmt.Sprintf("&%s", pattern))
	}
	return strings.Join(args, " ")
}

func (r *Rule) IsCommandEnabled(cmd string, cates []string) bool {
	if slices.Contains(r.DisallowedCommands, cmd) {
		return false
	}
	if slices.Contains(r.AllowedCommands, cmd) {
		return true
	}
	for _, cate := range cates {
		if slices.Contains(r.DisallowedCategories, cate) {
			return false
		}
	}
	for _, cate := range cates {
		if slices.Contains(r.Categories, cate) {
			return true
		}
	}
	return false
}

func (r *Rule) Validate(disableACL bool) error {
	for _, cate := range append(append([]string{}, r.Categories...), r.DisallowedCategories...) {
		if !slices.Contains(allowedCategories, cate) {
			return fmt.Errorf("unsupported category %s", cate)
		}
	}
	if len(r.Categories) == 0 && len(r.AllowedCommands) == 0 {
		return fmt.Errorf("at least one category or command should be enabled")
	}
	if len(r.KeyPatterns) == 0 && len(r.KeyReadPatterns) == 0 && len(r.KeyWritePatterns) == 0 && len(r.Channels) == 0 {
		return fmt.Errorf("at least one key pattern or channel pattern should be enabled")
	}
	if disableACL {
		if r.IsCommandEnabled("acl", []string{"all", "admin", "slow", "dangerous"}) {
			return fmt.Errorf("`acl` and it's sub commands are enabled")
		}
		for _, cmd := range r.AllowedCommands {
			if strings.HasPrefix(cmd, "acl|") {
				return fmt.Errorf("`acl` and it's sub commands are enabled")
			}
		}
	}
	return nil
}
