package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/playwright-community/playwright-go"
)

// This script explores the Tackle UI analysis wizard to identify target checkbox selectors

func main() {
	// Install Playwright if needed
	err := playwright.Install()
	if err != nil {
		log.Fatalf("Failed to install playwright: %v", err)
	}

	// Launch browser
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("Failed to start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		SlowMo:   playwright.Float(1000),
	})
	if err != nil {
		log.Fatalf("Failed to launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("Failed to create page: %v", err)
	}

	// Navigate to Tackle UI
	tackleURL := "http://localhost:8080"
	fmt.Printf("Navigating to %s...\n", tackleURL)
	_, err = page.Goto(tackleURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Navigate to Applications
	fmt.Println("Navigating to Applications page...")
	err = page.Click("a[href*='application']")
	if err != nil {
		log.Fatalf("Failed to click Applications: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Create a test application
	fmt.Println("Creating test application...")
	err = page.Click("button:has-text('Create')")
	if err != nil {
		log.Fatalf("Failed to click Create: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Fill name
	err = page.Fill("input[name='name']", "Target Test App")
	if err != nil {
		log.Fatalf("Failed to fill name: %v", err)
	}

	// Expand Source code section
	fmt.Println("Expanding Source code section...")
	err = page.Click("button:has-text('Source code')")
	if err != nil {
		log.Fatalf("Failed to expand Source code: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// Fill Git fields
	err = page.Fill("input[name='source.url']", "https://github.com/konveyor/tackle-testapp-public")
	if err != nil {
		log.Fatalf("Failed to fill source.url: %v", err)
	}

	err = page.Fill("input[name='source.branch']", "main")
	if err != nil {
		log.Fatalf("Failed to fill source.branch: %v", err)
	}

	// Let user manually submit the form and start analysis
	fmt.Println("\n=== MANUAL STEP REQUIRED ===")
	fmt.Println("Please manually:")
	fmt.Println("1. Click the 'Create' button to submit the form")
	fmt.Println("2. Wait for the application to be created")
	fmt.Println("3. Select the application checkbox")
	fmt.Println("4. Click the 'Analyze' button")
	fmt.Println("\nPress Enter when you see the Analysis Wizard...")
	fmt.Scanln()

	// Now we're in the analysis wizard - capture the HTML
	fmt.Println("\n=== ANALYSIS WIZARD OPENED ===")
	fmt.Println("Taking screenshot...")
	_, err = page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String("/tmp/analyze-wizard-step1.png"),
	})
	if err != nil {
		fmt.Printf("Failed to take screenshot: %v\n", err)
	}

	// Get the page HTML
	fmt.Println("Extracting HTML...")
	html, err := page.Content()
	if err != nil {
		log.Fatalf("Failed to get HTML: %v", err)
	}

	// Write to file
	err = os.WriteFile("/tmp/analyze-wizard-step1.html", []byte(html), 0644)
	if err != nil {
		log.Fatalf("Failed to write HTML: %v", err)
	}
	fmt.Println("HTML written to /tmp/analyze-wizard-step1.html")

	// Try to find target-related elements
	fmt.Println("\n=== Searching for target-related elements ===")

	// Search for elements containing "target" (case-insensitive)
	targetElements, err := page.Locator("text=/target/i").All()
	if err == nil {
		fmt.Printf("Found %d elements containing 'target'\n", len(targetElements))
		for i, elem := range targetElements {
			if i >= 10 {
				fmt.Printf("... and %d more\n", len(targetElements)-10)
				break
			}
			text, _ := elem.TextContent()
			fmt.Printf("  %d: %s\n", i+1, text)
		}
	}

	// Search for checkboxes
	fmt.Println("\n=== Searching for checkboxes ===")
	checkboxes, err := page.Locator("input[type='checkbox']").All()
	if err == nil {
		fmt.Printf("Found %d checkboxes\n", len(checkboxes))
		for i, cb := range checkboxes {
			if i >= 10 {
				fmt.Printf("... and %d more\n", len(checkboxes)-10)
				break
			}

			// Try to get attributes
			name, _ := cb.GetAttribute("name")
			id, _ := cb.GetAttribute("id")
			value, _ := cb.GetAttribute("value")
			ariaLabel, _ := cb.GetAttribute("aria-label")

			fmt.Printf("  %d: name=%s, id=%s, value=%s, aria-label=%s\n", i+1, name, id, value, ariaLabel)
		}
	}

	// Look for common target names
	fmt.Println("\n=== Searching for known targets ===")
	knownTargets := []string{"cloud-readiness", "linux", "quarkus", "rhr", "azure"}
	for _, target := range knownTargets {
		selector := fmt.Sprintf("text=%s", target)
		count, err := page.Locator(selector).Count()
		if err == nil && count > 0 {
			fmt.Printf("  Found '%s' (%d occurrences)\n", target, count)
		}
	}

	// Wait for user to inspect
	fmt.Println("\n=== Browser window is open - inspect manually ===")
	fmt.Println("Press Enter to continue and try clicking 'Next'...")
	fmt.Scanln()

	// Try clicking Next to see next step
	fmt.Println("Attempting to click Next button...")
	err = page.Click("button:has-text('Next')")
	if err != nil {
		fmt.Printf("Failed to click Next (might not exist): %v\n", err)
	} else {
		fmt.Println("Clicked Next successfully!")
		time.Sleep(2 * time.Second)

		// Take another screenshot
		_, err = page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String("/tmp/analyze-wizard-step2.png"),
		})
		if err == nil {
			fmt.Println("Screenshot saved to /tmp/analyze-wizard-step2.png")
		}

		// Get HTML again
		html, err = page.Content()
		if err == nil {
			os.WriteFile("/tmp/analyze-wizard-step2.html", []byte(html), 0644)
			fmt.Println("HTML written to /tmp/analyze-wizard-step2.html")
		}

		// Search for targets again
		fmt.Println("\n=== Step 2: Searching for target checkboxes ===")
		for _, target := range knownTargets {
			// Try different selector strategies
			selectors := []string{
				fmt.Sprintf("input[type='checkbox'][value='%s']", target),
				fmt.Sprintf("input[type='checkbox'][name*='%s']", target),
				fmt.Sprintf("label:has-text('%s') input[type='checkbox']", target),
				fmt.Sprintf("div:has-text('%s') input[type='checkbox']", target),
			}

			for _, sel := range selectors {
				count, _ := page.Locator(sel).Count()
				if count > 0 {
					fmt.Printf("  ✓ Found '%s' with selector: %s\n", target, sel)
					break
				}
			}
		}
	}

	fmt.Println("\nPress Enter to close browser...")
	fmt.Scanln()
}
