// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filter

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExpressionFilter evaluates filter expressions
type ExpressionFilter struct {
	expression string
	ast        *ASTNode
}

// ASTNode represents a node in the abstract syntax tree
type ASTNode struct {
	Type     NodeType
	Operator string
	Left     *ASTNode
	Right    *ASTNode
	Value    interface{}
	Field    string
}

// NodeType represents the type of AST node
type NodeType int

const (
	NodeTypeLiteral NodeType = iota
	NodeTypeField
	NodeTypeComparison
	NodeTypeLogical
	NodeTypeMacro
)

// NewExpressionFilter creates a new expression filter
func NewExpressionFilter(expression string) (*ExpressionFilter, error) {
	if expression == "" {
		return nil, fmt.Errorf("expression cannot be empty")
	}

	parser := &expressionParser{
		expression: strings.TrimSpace(expression),
		pos:        0,
	}

	ast, err := parser.parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse expression: %w", err)
	}

	return &ExpressionFilter{
		expression: expression,
		ast:        ast,
	}, nil
}

// Evaluate evaluates the expression against an observation
func (ef *ExpressionFilter) Evaluate(obs *unstructured.Unstructured) (bool, error) {
	if ef == nil || ef.ast == nil {
		return true, nil
	}

	result, err := ef.evaluateNode(ef.ast, obs)
	if err != nil {
		return false, err
	}

	if boolVal, ok := result.(bool); ok {
		return boolVal, nil
	}

	return false, fmt.Errorf("expression did not evaluate to boolean")
}

// evaluateNode recursively evaluates an AST node
func (ef *ExpressionFilter) evaluateNode(node *ASTNode, obs *unstructured.Unstructured) (interface{}, error) {
	switch node.Type {
	case NodeTypeLiteral:
		return node.Value, nil

	case NodeTypeField:
		return ef.getFieldValue(node.Field, obs)

	case NodeTypeComparison:
		return ef.evaluateComparison(node, obs)

	case NodeTypeLogical:
		return ef.evaluateLogical(node, obs)

	case NodeTypeMacro:
		return ef.evaluateMacro(node, obs)

	default:
		return false, fmt.Errorf("unknown node type: %v", node.Type)
	}
}

// getFieldValue extracts a field value from an observation
func (ef *ExpressionFilter) getFieldValue(fieldPath string, obs *unstructured.Unstructured) (interface{}, error) {
	// Support simple dot notation: spec.severity, spec.details.vulnerabilityID
	parts := strings.Split(fieldPath, ".")

	var current interface{} = obs.Object
	for _, part := range parts {
		if currentMap, ok := current.(map[string]interface{}); ok {
			val, exists := currentMap[part]
			if !exists {
				return nil, nil // Field doesn't exist
			}
			current = val
		} else {
			return nil, nil // Not a map, can't traverse
		}
	}

	return current, nil
}

// evaluateComparison evaluates a comparison operation
func (ef *ExpressionFilter) evaluateComparison(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	if node.Left == nil || node.Right == nil {
		return false, fmt.Errorf("comparison requires left and right operands")
	}

	leftVal, err := ef.evaluateNode(node.Left, obs)
	if err != nil {
		return false, err
	}

	rightVal, err := ef.evaluateNode(node.Right, obs)
	if err != nil {
		return false, err
	}

	return ef.evaluateComparisonOperator(node.Operator, leftVal, rightVal)
}

// evaluateLogical evaluates a logical operation (AND, OR, NOT)
func (ef *ExpressionFilter) evaluateLogical(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	return ef.evaluateLogicalOperator(node, obs)
}

// evaluateMacro evaluates a macro (e.g., is_critical, severity >= HIGH)
func (ef *ExpressionFilter) evaluateMacro(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	macroName := node.Field

	switch macroName {
	case "is_critical":
		severity, _ := ef.getFieldValue("spec.severity", obs)
		return ef.compareEqual(severity, "CRITICAL"), nil

	case "is_high":
		severity, _ := ef.getFieldValue("spec.severity", obs)
		return ef.compareEqual(severity, "HIGH"), nil

	case "is_security":
		category, _ := ef.getFieldValue("spec.category", obs)
		return ef.compareEqual(category, "security"), nil

	case "is_compliance":
		category, _ := ef.getFieldValue("spec.category", obs)
		return ef.compareEqual(category, "compliance"), nil

	default:
		return false, fmt.Errorf("unknown macro: %s", macroName)
	}
}

// Comparison helpers

func (ef *ExpressionFilter) compareEqual(left, right interface{}) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}

	// String comparison
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)
	return strings.EqualFold(leftStr, rightStr)
}

func (ef *ExpressionFilter) compareGreater(left, right interface{}) bool {
	// Try numeric comparison first
	leftNum, leftOk := ef.toNumber(left)
	rightNum, rightOk := ef.toNumber(right)
	if leftOk && rightOk {
		return leftNum > rightNum
	}

	// Try severity comparison
	if ef.isSeverity(left) && ef.isSeverity(right) {
		return ef.compareSeverity(left, right) > 0
	}

	// String comparison
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)
	return leftStr > rightStr
}

func (ef *ExpressionFilter) compareIn(value interface{}, list interface{}) bool {
	if listSlice, ok := list.([]interface{}); ok {
		for _, item := range listSlice {
			if ef.compareEqual(value, item) {
				return true
			}
		}
	}
	return false
}

func (ef *ExpressionFilter) compareContains(str, substr interface{}) bool {
	strVal := fmt.Sprintf("%v", str)
	substrVal := fmt.Sprintf("%v", substr)
	return strings.Contains(strings.ToLower(strVal), strings.ToLower(substrVal))
}

func (ef *ExpressionFilter) compareStartsWith(str, prefix interface{}) bool {
	strVal := fmt.Sprintf("%v", str)
	prefixVal := fmt.Sprintf("%v", prefix)
	return strings.HasPrefix(strings.ToLower(strVal), strings.ToLower(prefixVal))
}

func (ef *ExpressionFilter) compareEndsWith(str, suffix interface{}) bool {
	strVal := fmt.Sprintf("%v", str)
	suffixVal := fmt.Sprintf("%v", suffix)
	return strings.HasSuffix(strings.ToLower(strVal), strings.ToLower(suffixVal))
}

func (ef *ExpressionFilter) toNumber(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			return num, true
		}
	}
	return 0, false
}

func (ef *ExpressionFilter) isSeverity(val interface{}) bool {
	severityStr := strings.ToUpper(fmt.Sprintf("%v", val))
	severities := map[string]bool{
		"CRITICAL": true,
		"HIGH":     true,
		"MEDIUM":   true,
		"LOW":      true,
		"UNKNOWN":  true,
	}
	return severities[severityStr]
}

func (ef *ExpressionFilter) compareSeverity(left, right interface{}) int {
	severityLevels := map[string]int{
		"CRITICAL": 5,
		"HIGH":     4,
		"MEDIUM":   3,
		"LOW":      2,
		"UNKNOWN":  1,
	}

	leftStr := strings.ToUpper(fmt.Sprintf("%v", left))
	rightStr := strings.ToUpper(fmt.Sprintf("%v", right))

	leftLevel := severityLevels[leftStr]
	rightLevel := severityLevels[rightStr]

	return leftLevel - rightLevel
}

// evaluateComparisonOperator evaluates a comparison operator
func (ef *ExpressionFilter) evaluateComparisonOperator(op string, leftVal, rightVal interface{}) (bool, error) {
	switch op {
	case "=", "!=":
		return ef.evaluateEqualityOperator(op, leftVal, rightVal)
	case ">", ">=", "<", "<=":
		return ef.evaluateNumericComparisonOperator(op, leftVal, rightVal)
	case "IN", "NOT IN":
		return ef.evaluateInOperator(op, leftVal, rightVal)
	case "CONTAINS", "STARTS_WITH", "ENDS_WITH":
		return ef.evaluateStringOperator(op, leftVal, rightVal)
	case "EXISTS", "NOT EXISTS":
		return ef.evaluateExistenceOperator(op, leftVal)
	default:
		return false, fmt.Errorf("unknown comparison operator: %s", op)
	}
}

// evaluateEqualityOperator evaluates equality operators
func (ef *ExpressionFilter) evaluateEqualityOperator(op string, leftVal, rightVal interface{}) (bool, error) {
	result := ef.compareEqual(leftVal, rightVal)
	if op == "!=" {
		return !result, nil
	}
	return result, nil
}

// evaluateNumericComparisonOperator evaluates comparison operators (>, >=, <, <=)
func (ef *ExpressionFilter) evaluateNumericComparisonOperator(op string, leftVal, rightVal interface{}) (bool, error) {
	switch op {
	case ">":
		return ef.compareGreater(leftVal, rightVal), nil
	case ">=":
		eq := ef.compareEqual(leftVal, rightVal)
		gt := ef.compareGreater(leftVal, rightVal)
		return eq || gt, nil
	case "<":
		return ef.compareGreater(rightVal, leftVal), nil
	case "<=":
		eq := ef.compareEqual(leftVal, rightVal)
		gt := ef.compareGreater(rightVal, leftVal)
		return eq || gt, nil
	}
	return false, fmt.Errorf("unknown comparison operator: %s", op)
}

// evaluateInOperator evaluates IN/NOT IN operators
func (ef *ExpressionFilter) evaluateInOperator(op string, leftVal, rightVal interface{}) (bool, error) {
	result := ef.compareIn(leftVal, rightVal)
	if op == "NOT IN" {
		return !result, nil
	}
	return result, nil
}

// evaluateStringOperator evaluates string operators
func (ef *ExpressionFilter) evaluateStringOperator(op string, leftVal, rightVal interface{}) (bool, error) {
	switch op {
	case "CONTAINS":
		return ef.compareContains(leftVal, rightVal), nil
	case "STARTS_WITH":
		return ef.compareStartsWith(leftVal, rightVal), nil
	case "ENDS_WITH":
		return ef.compareEndsWith(leftVal, rightVal), nil
	}
	return false, fmt.Errorf("unknown string operator: %s", op)
}

// evaluateExistenceOperator evaluates EXISTS/NOT EXISTS operators
func (ef *ExpressionFilter) evaluateExistenceOperator(op string, leftVal interface{}) (bool, error) {
	exists := leftVal != nil
	if op == "NOT EXISTS" {
		return !exists, nil
	}
	return exists, nil
}

// evaluateLogicalOperator evaluates a logical operator
func (ef *ExpressionFilter) evaluateLogicalOperator(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	switch node.Operator {
	case "AND":
		return ef.evaluateAND(node, obs)
	case "OR":
		return ef.evaluateOR(node, obs)
	case "NOT":
		return ef.evaluateNOT(node, obs)
	default:
		return false, fmt.Errorf("unknown logical operator: %s", node.Operator)
	}
}

// evaluateAND evaluates AND operation
func (ef *ExpressionFilter) evaluateAND(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	if node.Left == nil || node.Right == nil {
		return false, fmt.Errorf("AND requires left and right operands")
	}
	leftVal, err := ef.evaluateNode(node.Left, obs)
	if err != nil {
		return false, err
	}
	if leftBool, ok := leftVal.(bool); ok && !leftBool {
		return false, nil // Short-circuit
	}
	rightVal, err := ef.evaluateNode(node.Right, obs)
	if err != nil {
		return false, err
	}
	if rightBool, ok := rightVal.(bool); ok {
		return rightBool, nil
	}
	return false, fmt.Errorf("logical operation requires boolean operands")
}

// evaluateOR evaluates OR operation
func (ef *ExpressionFilter) evaluateOR(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	if node.Left == nil || node.Right == nil {
		return false, fmt.Errorf("OR requires left and right operands")
	}
	leftVal, err := ef.evaluateNode(node.Left, obs)
	if err != nil {
		return false, err
	}
	if leftBool, ok := leftVal.(bool); ok && leftBool {
		return true, nil // Short-circuit
	}
	rightVal, err := ef.evaluateNode(node.Right, obs)
	if err != nil {
		return false, err
	}
	if rightBool, ok := rightVal.(bool); ok {
		return rightBool, nil
	}
	return false, fmt.Errorf("logical operation requires boolean operands")
}

// evaluateNOT evaluates NOT operation
func (ef *ExpressionFilter) evaluateNOT(node *ASTNode, obs *unstructured.Unstructured) (bool, error) {
	if node.Left == nil {
		return false, fmt.Errorf("NOT requires left operand")
	}
	leftVal, err := ef.evaluateNode(node.Left, obs)
	if err != nil {
		return false, err
	}
	if leftBool, ok := leftVal.(bool); ok {
		return !leftBool, nil
	}
	return false, fmt.Errorf("NOT requires boolean operand")
}
