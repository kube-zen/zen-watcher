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
	"unicode"
)

// expressionParser parses filter expressions into AST
type expressionParser struct {
	expression string
	pos        int
}

// parse parses the expression and returns an AST
func (p *expressionParser) parse() (*ASTNode, error) {
	if p.pos >= len(p.expression) {
		return nil, fmt.Errorf("empty expression")
	}

	node, err := p.parseExpression()
	if err != nil {
		return nil, err
	}

	// Check for trailing tokens
	p.skipWhitespace()
	if p.pos < len(p.expression) {
		return nil, fmt.Errorf("unexpected token at position %d: %c", p.pos, p.expression[p.pos])
	}

	return node, nil
}

// parseExpression parses a logical expression (OR has lowest precedence)
func (p *expressionParser) parseExpression() (*ASTNode, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	for p.pos < len(p.expression) {
		if p.peekToken("OR") {
			p.consumeToken("OR")
			right, err := p.parseAndExpression()
			if err != nil {
				return nil, err
			}
			left = &ASTNode{
				Type:     NodeTypeLogical,
				Operator: "OR",
				Left:     left,
				Right:    right,
			}
		} else {
			break
		}
		p.skipWhitespace()
	}

	return left, nil
}

// parseAndExpression parses AND expressions
func (p *expressionParser) parseAndExpression() (*ASTNode, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	for p.pos < len(p.expression) {
		if p.peekToken("AND") {
			p.consumeToken("AND")
			right, err := p.parseNotExpression()
			if err != nil {
				return nil, err
			}
			left = &ASTNode{
				Type:     NodeTypeLogical,
				Operator: "AND",
				Left:     left,
				Right:    right,
			}
		} else {
			break
		}
		p.skipWhitespace()
	}

	return left, nil
}

// parseNotExpression parses NOT expressions
func (p *expressionParser) parseNotExpression() (*ASTNode, error) {
	p.skipWhitespace()
	if p.peekToken("NOT") {
		p.consumeToken("NOT")
		operand, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return &ASTNode{
			Type:     NodeTypeLogical,
			Operator: "NOT",
			Left:     operand,
		}, nil
	}

	return p.parseComparison()
}

// parseComparison parses comparison expressions
func (p *expressionParser) parseComparison() (*ASTNode, error) {
	// Check for parentheses
	if p.peekChar() == '(' {
		p.consumeChar('(')
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.peekChar() != ')' {
			return nil, fmt.Errorf("expected ')' at position %d", p.pos)
		}
		p.consumeChar(')')
		return expr, nil
	}

	// Check for macro
	if p.peekToken("is_") {
		macroName := p.parseIdentifier()
		return &ASTNode{
			Type:  NodeTypeMacro,
			Field: macroName,
		}, nil
	}

	// Parse left operand (field or literal)
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Check for comparison operators
	operators := []string{"NOT IN", "NOT EXISTS", ">=", "<=", "!=", "=", ">", "<", "IN", "EXISTS", "CONTAINS", "STARTS_WITH", "ENDS_WITH"}
	var operator string

	for _, op := range operators {
		if p.peekToken(op) {
			operator = op
			p.consumeToken(op)
			break
		}
	}

	if operator == "" {
		// No operator - this is just a field or literal (evaluates to truthy)
		return left, nil
	}

	p.skipWhitespace()

	// Parse right operand
	right, err := p.parseOperand()
	if err != nil {
		return nil, err
	}

	return &ASTNode{
		Type:     NodeTypeComparison,
		Operator: operator,
		Left:     left,
		Right:    right,
	}, nil
}

// parseOperand parses an operand (field, literal, or list)
func (p *expressionParser) parseOperand() (*ASTNode, error) {
	p.skipWhitespace()

	// Check for list [item1, item2, ...]
	if p.peekChar() == '[' {
		return p.parseList()
	}

	// Check for string literal
	if p.peekChar() == '"' || p.peekChar() == '\'' {
		return p.parseStringLiteral()
	}

	// Check for number
	if unicode.IsDigit(rune(p.peekChar())) || p.peekChar() == '-' {
		return p.parseNumber()
	}

	// Check for boolean
	if p.peekToken("true") {
		p.consumeToken("true")
		return &ASTNode{
			Type:  NodeTypeLiteral,
			Value: true,
		}, nil
	}
	if p.peekToken("false") {
		p.consumeToken("false")
		return &ASTNode{
			Type:  NodeTypeLiteral,
			Value: false,
		}, nil
	}

	// Must be a field path
	fieldPath := p.parseFieldPath()
	return &ASTNode{
		Type:  NodeTypeField,
		Field: fieldPath,
	}, nil
}

// parseList parses a list literal [item1, item2, ...]
func (p *expressionParser) parseList() (*ASTNode, error) {
	p.consumeChar('[')
	p.skipWhitespace()

	var items []interface{}
	for p.pos < len(p.expression) && p.peekChar() != ']' {
		item, err := p.parseListItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)

		p.skipWhitespace()
		if p.peekChar() == ',' {
			p.consumeChar(',')
			p.skipWhitespace()
		} else {
			break
		}
	}

	if p.peekChar() != ']' {
		return nil, fmt.Errorf("expected ']' at position %d", p.pos)
	}
	p.consumeChar(']')

	return &ASTNode{
		Type:  NodeTypeLiteral,
		Value: items,
	}, nil
}

// parseListItem parses a single list item
func (p *expressionParser) parseListItem() (interface{}, error) {
	p.skipWhitespace()

	// String literal
	if p.peekChar() == '"' || p.peekChar() == '\'' {
		node, err := p.parseStringLiteral()
		if err != nil {
			return nil, err
		}
		return node.Value, nil
	}

	// Number
	if unicode.IsDigit(rune(p.peekChar())) || p.peekChar() == '-' {
		node, err := p.parseNumber()
		if err != nil {
			return nil, err
		}
		return node.Value, nil
	}

	// Identifier (treated as string)
	ident := p.parseIdentifier()
	return ident, nil
}

// parseStringLiteral parses a string literal
func (p *expressionParser) parseStringLiteral() (*ASTNode, error) {
	quote := p.peekChar()
	if quote != '"' && quote != '\'' {
		return nil, fmt.Errorf("expected string literal at position %d", p.pos)
	}
	p.consumeChar(quote)

	var sb strings.Builder
	for p.pos < len(p.expression) && p.peekChar() != quote {
		if p.peekChar() == '\\' {
			p.pos++
			if p.pos >= len(p.expression) {
				return nil, fmt.Errorf("unexpected end of string at position %d", p.pos)
			}
			switch p.peekChar() {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case '\\':
				sb.WriteRune('\\')
			case quote:
				sb.WriteRune(rune(quote))
			default:
				sb.WriteRune(rune(p.peekChar()))
			}
			p.pos++
		} else {
			sb.WriteRune(rune(p.peekChar()))
			p.pos++
		}
	}

	if p.pos >= len(p.expression) || p.peekChar() != quote {
		return nil, fmt.Errorf("unterminated string literal at position %d", p.pos)
	}
	p.consumeChar(quote)

	return &ASTNode{
		Type:  NodeTypeLiteral,
		Value: sb.String(),
	}, nil
}

// parseNumber parses a number literal
func (p *expressionParser) parseNumber() (*ASTNode, error) {
	start := p.pos
	negative := false

	if p.peekChar() == '-' {
		negative = true
		p.pos++
	}

	for p.pos < len(p.expression) && (unicode.IsDigit(rune(p.peekChar())) || p.peekChar() == '.') {
		p.pos++
	}

	numStr := p.expression[start:p.pos]
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number at position %d: %s", start, numStr)
	}

	if negative {
		num = -num
	}

	return &ASTNode{
		Type:  NodeTypeLiteral,
		Value: num,
	}, nil
}

// parseFieldPath parses a field path (e.g., spec.severity, spec.details.vulnerabilityID)
func (p *expressionParser) parseFieldPath() string {
	var parts []string
	start := p.pos

	for p.pos < len(p.expression) {
		char := p.peekChar()
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			p.pos++
		} else if char == '.' && len(parts) == 0 {
			// First part before dot
			parts = append(parts, p.expression[start:p.pos])
			p.pos++
			start = p.pos
		} else if char == '.' && len(parts) > 0 {
			// Subsequent part
			parts = append(parts, p.expression[start:p.pos])
			p.pos++
			start = p.pos
		} else {
			break
		}
	}

	if start < p.pos {
		parts = append(parts, p.expression[start:p.pos])
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, ".")
}

// parseIdentifier parses an identifier
func (p *expressionParser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.expression) {
		char := p.peekChar()
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			p.pos++
		} else {
			break
		}
	}
	return p.expression[start:p.pos]
}

// Helper methods

func (p *expressionParser) peekChar() byte {
	if p.pos >= len(p.expression) {
		return 0
	}
	return p.expression[p.pos]
}

func (p *expressionParser) consumeChar(expected byte) {
	if p.pos >= len(p.expression) || p.expression[p.pos] != expected {
		panic(fmt.Sprintf("expected '%c' at position %d", expected, p.pos))
	}
	p.pos++
}

func (p *expressionParser) peekToken(token string) bool {
	if p.pos+len(token) > len(p.expression) {
		return false
	}
	substr := p.expression[p.pos : p.pos+len(token)]
	if strings.EqualFold(substr, token) {
		// Check that it's not part of a larger identifier
		if p.pos+len(token) < len(p.expression) {
			nextChar := p.expression[p.pos+len(token)]
			if (nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z') || (nextChar >= '0' && nextChar <= '9') || nextChar == '_' {
				return false
			}
		}
		return true
	}
	return false
}

func (p *expressionParser) consumeToken(token string) {
	if !p.peekToken(token) {
		panic(fmt.Sprintf("expected token '%s' at position %d", token, p.pos))
	}
	p.pos += len(token)
}

func (p *expressionParser) skipWhitespace() {
	for p.pos < len(p.expression) && (p.peekChar() == ' ' || p.peekChar() == '\t' || p.peekChar() == '\n' || p.peekChar() == '\r') {
		p.pos++
	}
}
