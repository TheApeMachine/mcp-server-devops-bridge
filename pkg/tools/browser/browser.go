package browser

import (
	"context"
	"encoding/base64"
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
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

type BrowserTool struct {
	handle mcp.Tool
}

func NewBrowserTool() core.Tool {
	return &BrowserTool{
		handle: mcp.NewTool(
			"browser",
			mcp.WithDescription("Interact with the web"),
			mcp.WithString("url", mcp.Description("The URL to navigate to")),
			mcp.WithString("javascript", mcp.Description("The JavaScript to execute")),
			mcp.WithString("action", mcp.Description("The action to perform")),
			mcp.WithString("hotkeys", mcp.Description("The hotkeys to send")),
			mcp.WithString("screenshot", mcp.Description("Whether to take a screenshot")),
		),
	}
}

func (tool *BrowserTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *BrowserTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var (
		url        string
		javascript string
		action     string
		hotkeys    string
		ok         bool
	)
	browser := NewBrowser()

	if err := browser.StartSession(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start browser session: %v", err)), nil
	}

	defer browser.Close()

	if url, ok = request.Params.Arguments["url"].(string); ok {
		if err := browser.Navigate(url); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to navigate to URL: %v", err)), nil
		}
	}

	if javascript, ok = request.Params.Arguments["javascript"].(string); ok {
		result := browser.ExecuteScript(javascript)
		return mcp.NewToolResultText(result), nil
	}

	if action, ok = request.Params.Arguments["action"].(string); ok {
		if err := browser.handleAction(action); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to perform action: %v", err)), nil
		}
	}

	if hotkeys, ok = request.Params.Arguments["hotkeys"].(string); ok {
		if err := browser.handleHotkeys(hotkeys); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to send hotkeys: %v", err)), nil
		}
	}

	if _, ok = request.Params.Arguments["screenshot"].(bool); ok {
		img, err := browser.TakeScreenshot()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to take screenshot: %v", err)), nil
		}
		// Convert screenshot bytes to base64 string and provide required params
		base64Image := base64.StdEncoding.EncodeToString(img)
		return mcp.NewToolResultImage(base64Image, "image/png", "screenshot.png"), nil
	}

	return mcp.NewToolResultText("Browser session completed successfully"), nil
}

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
	Data    any
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
		Headless(false).
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
