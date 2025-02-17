package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type BrowserArgs struct {
	URL        string
	Selector   string
	Timeout    int
	Screenshot bool
}

type Browser struct {
	Operation      string
	URL            string
	Javascript     string
	ExtractorType  string
	ExtractorName  string
	CustomScript   string
	HelperNames    []string
	instance       *rod.Browser
	page           *rod.Page
	currentElement *rod.Element
	history        []BrowseAction
	proxy          *url.URL
	err            error
}

type BrowseAction struct {
	Type    string
	Data    interface{}
	Result  string
	Time    time.Time
	Success bool
}

type NetworkRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    string
}

type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Expires  time.Time
	Secure   bool
	HTTPOnly bool
}

// Add new types for enhanced functionality
type BrowserResult struct {
	Content    string
	Screenshot []byte
	Status     string
	Error      string
}

func NewBrowser() *Browser {
	return &Browser{
		history: make([]BrowseAction, 0),
	}
}

func (browser *Browser) Name() string {
	return "browser"
}

func (browser *Browser) Description() string {
	return "Interact with the web"
}

func (browser *Browser) Initialize() error {
	return nil
}

func (browser *Browser) Connect(ctx context.Context, conn io.ReadWriteCloser) error {
	return nil
}

func (browser *Browser) Use(ctx context.Context, args map[string]any) string {
	var (
		result string
		err    error
	)

	if result, err = browser.Run(args); err != nil {
		return err.Error()
	}

	return result
}

// SetProxy configures a proxy for the browser
func (browser *Browser) SetProxy(proxyURL string) {
	browser.proxy, browser.err = url.Parse(proxyURL)
}

// StartSession initializes a new browsing session with stealth mode
func (browser *Browser) StartSession() error {
	log.Info("Starting browser session")

	// Set custom user directory to avoid permission issues
	userDataDir := os.TempDir() + "/rod-user-data"
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create user data directory: %v", err)
	}

	l := launcher.New().
		UserDataDir(userDataDir).
		Headless(true). // Change to true for server environments
		Set("disable-web-security", "").
		Set("disable-setuid-sandbox", "").
		Set("no-sandbox", "")

	// Try to detect browser binary
	if path, exists := launcher.LookPath(); exists {
		l = l.Bin(path)
	} else {
		// Attempt to use system Chrome/Chromium
		for _, bin := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", // macOS Chrome
			"/Applications/Chromium.app/Contents/MacOS/Chromium",           // macOS Chromium
			"/usr/bin/google-chrome",                                       // Linux Chrome
			"/usr/bin/chromium",                                            // Linux Chromium
			"/usr/bin/chromium-browser",                                    // Alternative Linux Chromium
		} {
			if _, err := os.Stat(bin); err == nil {
				l = l.Bin(bin)
				break
			}
		}
	}

	if browser.proxy != nil {
		l.Proxy(browser.proxy.String())
	}

	debugURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %v", err)
	}

	browser.instance = rod.New().
		ControlURL(debugURL).
		MustConnect()

	// Create a new stealth page
	browser.page, err = stealth.Page(browser.instance)
	if err != nil {
		return fmt.Errorf("failed to create stealth page: %v", err)
	}

	browser.instance.MustIgnoreCertErrors(true)

	return nil
}

// Navigate goes to a URL and waits for the page to load
func (browser *Browser) Navigate(url string) error {
	log.Info("Navigating", "url", url)

	// Instead of creating a new page, use the existing stealth page
	if err := browser.page.Navigate(url); err != nil {
		log.Error("Failed to navigate", "error", err)
		return err
	}

	// Wait for network to be idle and page to be fully loaded
	if err := browser.page.WaitLoad(); err != nil {
		log.Error("Failed to wait for page load", "error", err)
		return err
	}

	// Additional wait for dynamic content
	if err := browser.page.WaitIdle(5); err != nil {
		log.Error("Failed to wait for page idle", "error", err)
		return err
	}

	return nil
}

// ExecuteScript runs custom JavaScript and returns the result
func (browser *Browser) ExecuteScript(script string) string {
	if script == "" {
		log.Warn("No script provided")
		return ""
	}

	// Ensure the script is a function that returns a string
	if !strings.Contains(script, "return") && !strings.Contains(script, "=>") {
		log.Warn("Script must be a function that returns a string")
		return ""
	}

	// Execute the script and get the result
	result := browser.page.MustEval(script).Str()

	// Limit the result size to avoid context overflow (e.g., 10KB)
	const maxResultSize = 10 * 1024
	if len(result) > maxResultSize {
		log.Warn("Script result exceeded size limit, truncating", "size", len(result), "limit", maxResultSize)
		return result[:maxResultSize] + "\n... (truncated)"
	}

	return result
}

// TakeScreenshot takes a screenshot of the current page
func (browser *Browser) TakeScreenshot() ([]byte, error) {
	if browser.page == nil {
		return nil, fmt.Errorf("no active page")
	}

	quality := 100

	return browser.page.Screenshot(true, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatPng,
		Quality: &quality,
		Clip: &proto.PageViewport{
			X:      0,
			Y:      0,
			Width:  1920,
			Height: 1080,
			Scale:  1,
		},
	})
}

// WaitForSelector waits for an element to be visible
func (browser *Browser) WaitForSelector(selector string, timeout int) error {
	if timeout == 0 {
		timeout = 30 // default timeout
	}
	return browser.page.Timeout(time.Duration(timeout) * time.Second).MustElement(selector).WaitVisible()
}

/*
Run the Browser and react to the arguments that were provided by the Large Language Model
*/
func (browser *Browser) Run(args map[string]any) (string, error) {
	result := &BrowserResult{
		Status: "success",
	}

	if proxyURL, ok := args["proxy"].(string); ok {
		browser.SetProxy(proxyURL)
	}

	// Only start a new session if one doesn't exist
	if browser.instance == nil {
		if err := browser.StartSession(); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return "", err
		}
	}

	// Handle navigation only if URL is provided
	if url, ok := args["url"].(string); ok {
		if err := browser.Navigate(url); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return "", err
		}
	}

	if script, ok := args["javascript"].(string); ok {
		result.Content = browser.ExecuteScript(script)
	}

	// Handle actions
	if action, ok := args["action"].(string); ok && action != "" {
		if err := browser.handleAction(action); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return "", err
		}
	}

	// Handle hotkeys
	if hotkeys, ok := args["hotkeys"].(string); ok && hotkeys != "" {
		if err := browser.handleHotkeys(hotkeys); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			return "", err
		}
	}

	// Take screenshot if requested
	if screenshot, ok := args["screenshot"].(bool); ok && screenshot {
		if bytes, err := browser.TakeScreenshot(); err == nil {
			result.Screenshot = bytes
		} else {
			result.Status = "warning"
			result.Error = fmt.Sprintf("screenshot failed: %v", err)
		}
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultJSON), nil
}

// Helper method to handle actions
func (browser *Browser) handleAction(action string) error {
	if browser.currentElement == nil {
		return fmt.Errorf("no element selected for action: %s", action)
	}

	switch action {
	case "click":
		browser.currentElement.MustClick()
	case "scroll":
		browser.page.Mouse.Scroll(0, 400, 1)
	case "hover":
		browser.currentElement.MustHover()
	case "keypress":
		browser.handleHotkeys(action)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	return nil
}

// Helper method to handle hotkeys
func (browser *Browser) handleHotkeys(hotkeys string) error {
	if browser.currentElement == nil {
		return fmt.Errorf("no element selected for hotkeys")
	}

	keys := make([]input.Key, len(hotkeys))
	for i, r := range hotkeys {
		keys[i] = input.Key(r)
	}
	browser.currentElement.MustType(keys...)
	return nil
}

func (browser *Browser) Close() error {
	if browser.instance != nil {
		return browser.instance.Close()
	}
	return nil
}

func addBrowserTools(s *server.MCPServer) {
	// Add JavaScript Format Prompt
	s.AddPrompt(mcp.NewPrompt("javascript_format",
		mcp.WithPromptDescription("Helper for formatting JavaScript to extract content from web pages"),
		mcp.WithArgument("extraction_type",
			mcp.ArgumentDescription("Type of content to extract (e.g., 'main_content', 'article_text', 'links', 'metadata')"),
			mcp.RequiredArgument(),
		),
		mcp.WithArgument("custom_selectors",
			mcp.ArgumentDescription("Optional comma-separated list of CSS selectors to target specific elements"),
		),
	), handleJavaScriptFormatPrompt)

	// Navigate Tool
	navigateTool := mcp.NewTool("browser_navigate",
		mcp.WithDescription("Navigate to a URL in the browser"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to navigate to"),
		),
	)
	s.AddTool(navigateTool, handleBrowserNavigate)

	// Execute JavaScript Tool
	executeScriptTool := mcp.NewTool("browser_execute_script",
		mcp.WithDescription("Execute JavaScript in the browser"),
		mcp.WithString("script",
			mcp.Required(),
			mcp.Description("JavaScript code to execute"),
		),
	)
	s.AddTool(executeScriptTool, handleBrowserExecuteScript)

	// Take Screenshot Tool
	screenshotTool := mcp.NewTool("browser_screenshot",
		mcp.WithDescription("Take a screenshot of the current page"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name for the screenshot file"),
		),
	)
	s.AddTool(screenshotTool, handleBrowserScreenshot)

	// // Wait For Element Tool
	// waitForElementTool := mcp.NewTool("browser_wait_for_element",
	// 	mcp.WithDescription("Wait for an element to be visible on the page"),
	// 	mcp.WithString("selector",
	// 		mcp.Required(),
	// 		mcp.Description("CSS selector for the element"),
	// 	),
	// 	mcp.WithNumber("timeout",
	// 		mcp.Description("Timeout in seconds (default: 30)"),
	// 	),
	// )
	// s.AddTool(waitForElementTool, handleBrowserWaitForElement)
}

// Handler functions for browser tools
func handleBrowserNavigate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url := request.Params.Arguments["url"].(string)
	browser := NewBrowser()

	if err := browser.StartSession(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start browser session: %v", err)), nil
	}
	defer browser.Close()

	if err := browser.Navigate(url); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to navigate: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully navigated to %s", url)), nil
}

func handleBrowserExecuteScript(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	script := request.Params.Arguments["script"].(string)
	browser := NewBrowser()

	if err := browser.StartSession(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start browser session: %v", err)), nil
	}
	defer browser.Close()

	// Ensure we have a URL to execute the script on
	if browser.page == nil {
		return mcp.NewToolResultError("No page loaded. Please navigate to a URL first."), nil
	}

	// Execute the script
	result := browser.ExecuteScript(script)
	if result == "" {
		return mcp.NewToolResultError("Script execution failed or returned empty result"), nil
	}

	return mcp.NewToolResultText(result), nil
}

func handleBrowserScreenshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.Params.Arguments["name"].(string)
	browser := NewBrowser()

	if err := browser.StartSession(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start browser session: %v", err)), nil
	}
	defer browser.Close()

	screenshot, err := browser.TakeScreenshot()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to take screenshot: %v", err)), nil
	}

	// Convert screenshot to base64 for transmission
	result := map[string]interface{}{
		"name":     name,
		"data":     screenshot,
		"encoding": "base64",
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to encode screenshot: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func handleBrowserWaitForElement(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	selector := request.Params.Arguments["selector"].(string)
	timeout := 30 // default timeout

	if timeoutArg, ok := request.Params.Arguments["timeout"].(float64); ok {
		timeout = int(timeoutArg)
	}

	browser := NewBrowser()

	if err := browser.StartSession(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start browser session: %v", err)), nil
	}
	defer browser.Close()

	if err := browser.WaitForSelector(selector, timeout); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to find element: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Element '%s' found", selector)), nil
}

// Handler for JavaScript format prompt
func handleJavaScriptFormatPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	extractionTypeVal, exists := request.Params.Arguments["extraction_type"]
	if !exists {
		return nil, fmt.Errorf("extraction_type is required")
	}
	extractionType := extractionTypeVal

	customSelectors, hasCustomSelectors := request.Params.Arguments["custom_selectors"]

	var script string
	switch extractionType {
	case "main_content":
		script = `() => {
			const out = new Set();
			const consider = [
				"article",
				"main",
				"[role='main']",
				".content",
				"#content",
				".post",
				".article",
			];

			let mainContent = document.body;
			for (const selector of consider) {
				const container = document.querySelector(selector);
				if (container) {
					mainContent = container;
					break;
				}
			}

			for (const element of mainContent.querySelectorAll("p, h1, h2, h3, h4, h5, h6")) {
				out.add(element.innerText
					.split("\n")
					.map(line => line.trim())
					.filter(line => line.length > 5)
					.join("\n")
				);
			}

			return Array.from(out.values()).join("\n");
		}`

	case "article_text":
		script = `() => {
			const article = document.querySelector("article") || document.querySelector("[role='article']");
			if (!article) return "";
			return Array.from(article.querySelectorAll("p"))
				.map(p => p.innerText.trim())
				.filter(text => text.length > 0)
				.join("\n\n");
		}`

	case "links":
		script = `() => {
			return Array.from(document.links)
				.map(link => ({
					text: link.innerText.trim(),
					href: link.href
				}))
				.filter(link => link.text.length > 0)
				.map(link => link.text + ": " + link.href)
				.join("\n");
		}`

	case "metadata":
		script = `() => {
			const metadata = {
				title: document.title,
				description: document.querySelector("meta[name='description']")?.content || "",
				keywords: document.querySelector("meta[name='keywords']")?.content || "",
				author: document.querySelector("meta[name='author']")?.content || "",
				ogTitle: document.querySelector("meta[property='og:title']")?.content || "",
				ogDescription: document.querySelector("meta[property='og:description']")?.content || "",
			};
			return JSON.stringify(metadata, null, 2);
		}`

	default:
		if hasCustomSelectors {
			// Create a script using custom selectors
			customSelectorsStr := customSelectors
			selectors := strings.Split(customSelectorsStr, ",")
			for i := range selectors {
				selectors[i] = strings.TrimSpace(selectors[i])
			}
			script = fmt.Sprintf(`() => {
				const selectors = %s;
				const results = new Set();
				
				for (const selector of selectors) {
					const elements = document.querySelectorAll(selector);
					for (const el of elements) {
						results.add(el.innerText.trim());
					}
				}
				
				return Array.from(results)
					.filter(text => text.length > 0)
					.join("\n\n");
			}`, fmt.Sprintf("[\"%s\"]", strings.Join(selectors, "\", \"")))
		} else {
			return nil, fmt.Errorf("Unknown extraction type and no custom selectors provided")
		}
	}

	return mcp.NewGetPromptResult(
		"JavaScript Content Extraction Helper",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				"system",
				mcp.NewTextContent("You are a JavaScript expert that helps extract content from web pages. All scripts must be functions that return strings to avoid overwhelming the context window."),
			),
			mcp.NewPromptMessage(
				"assistant",
				mcp.NewTextContent(fmt.Sprintf("Here's a script to extract %s from the page:\n\n```javascript\n%s\n```\n\nThis script:\n1. Returns content as a string\n2. Handles empty/missing elements\n3. Cleans and formats the text\n4. Avoids returning raw HTML", extractionType, script)),
			),
		},
	), nil
}
