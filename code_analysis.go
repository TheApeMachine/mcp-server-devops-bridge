package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmc/langchaingo/llms/openai"
)

// CodeAnalyzer provides code analysis capabilities
type CodeAnalyzer struct {
	llm *openai.LLM
}

// NewCodeAnalyzer creates a new CodeAnalyzer instance
func NewCodeAnalyzer() (*CodeAnalyzer, error) {
	llm, err := openai.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OpenAI client: %v", err)
	}

	return &CodeAnalyzer{
		llm: llm,
	}, nil
}

// AnalyzeCode performs comprehensive code analysis
func (ca *CodeAnalyzer) AnalyzeCode(ctx context.Context, path string) (*CodeAnalysis, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Determine language from file extension
	language := determineLanguage(path)

	// Default embedding size for analysis
	embedding := make([]float32, 384)

	// Analyze code complexity
	complexity, err := ca.analyzeComplexity(ctx, string(content), language)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze complexity: %v", err)
	}

	// Analyze potential bugs
	bugProb, err := ca.analyzeBugProbability(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze bug probability: %v", err)
	}

	// Analyze security
	security, err := ca.analyzeSecurityIssues(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to analyze security: %v", err)
	}

	return &CodeAnalysis{
		Embedding:      embedding,
		Complexity:     complexity,
		BugProbability: bugProb,
		Security:       security,
	}, nil
}

// CodeAnalysis represents the results of code analysis
type CodeAnalysis struct {
	Embedding      []float32         // Vector representation of the code
	Complexity     float32           // Cyclomatic complexity estimate
	BugProbability float32           // Probability of containing bugs
	Security       map[string]string // Security issues found
}

// analyzeComplexity estimates the cyclomatic complexity of the code
func (ca *CodeAnalyzer) analyzeComplexity(ctx context.Context, code, language string) (float32, error) {
	prompt := fmt.Sprintf(`Analyze the following %s code and estimate its cyclomatic complexity.
Consider control flow statements, loops, conditionals, and function calls.
Provide a single number representing the complexity score.

Code:
%s

Respond with only a number between 1 and 100:`, language, code)

	response, err := ca.llm.Call(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze complexity: %v", err)
	}

	var complexity float32
	_, err = fmt.Sscanf(response, "%f", &complexity)
	if err != nil {
		return 0, fmt.Errorf("failed to parse complexity score: %v", err)
	}

	return complexity, nil
}

// analyzeBugProbability estimates the likelihood of bugs in the code
func (ca *CodeAnalyzer) analyzeBugProbability(ctx context.Context, code string) (float32, error) {
	prompt := fmt.Sprintf(`Analyze the following code for potential bugs and code quality issues.
Consider error handling, null checks, resource management, and common programming mistakes.
Provide a probability score where:
0 = Very low probability of bugs
1 = Very high probability of bugs

Code:
%s

Respond with only a number between 0 and 1:`, code)

	response, err := ca.llm.Call(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("failed to analyze bug probability: %v", err)
	}

	var probability float32
	_, err = fmt.Sscanf(response, "%f", &probability)
	if err != nil {
		return 0, fmt.Errorf("failed to parse bug probability: %v", err)
	}

	return probability, nil
}

// analyzeSecurityIssues identifies potential security vulnerabilities
func (ca *CodeAnalyzer) analyzeSecurityIssues(ctx context.Context, code string) (map[string]string, error) {
	prompt := fmt.Sprintf(`Analyze the following code for potential security vulnerabilities.
Consider input validation, authentication, authorization, data exposure, and other security concerns.
List each potential security issue in the format:
ISSUE: Description

Code:
%s

List security issues:`, code)

	response, err := ca.llm.Call(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze security issues: %v", err)
	}

	// Parse security issues from response
	issues := make(map[string]string)
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				issue := strings.TrimSpace(parts[0])
				description := strings.TrimSpace(parts[1])
				issues[issue] = description
			}
		}
	}

	return issues, nil
}

// determineLanguage identifies the programming language from file extension
func determineLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".cs":
		return "C#"
	case ".cpp", ".cc", ".cxx":
		return "C++"
	case ".rs":
		return "Rust"
	default:
		return "Unknown"
	}
}
