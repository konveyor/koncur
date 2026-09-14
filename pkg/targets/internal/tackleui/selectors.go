package tackleui

// Selector constants for Tackle UI elements
// Based on Tackle UI version 2.0.0 exploration

// Navigation selectors
const (
	// SelectorApplicationsNav is the main Applications navigation link
	SelectorApplicationsNav = "a[href*='application']"

	// SelectorAdministratorNav is the Administrator navigation menu
	SelectorAdministratorNav = "a:has-text('Administrator')"

	// SelectorCredentialsNav is the Credentials navigation link
	SelectorCredentialsNav = "a[href='/identities']"
)

// Application management selectors
const (
	// SelectorCreateAppButton is the "Create" button on applications page
	SelectorCreateAppButton = "button:has-text('Create')"

	// SelectorCloseModal closes the create application modal
	SelectorCloseModal = "[aria-label='Close']"
)

// Form field selectors - Basic Information section
const (
	// SelectorFormName is the application name input
	SelectorFormName = "input[name='name']"

	// SelectorFormDescription is the application description textarea
	SelectorFormDescription = "textarea[name='description']"

	// SelectorFormComments is the comments textarea
	SelectorFormComments = "textarea[name='comments']"

	// SelectorFormBusinessService is the business service dropdown
	SelectorFormBusinessService = "#business-service-toggle-select-typeahead"

	// SelectorFormTags is the tags multiselect
	SelectorFormTags = "[aria-label='tags-select-toggle']"

	// SelectorFormOwner is the owner dropdown
	SelectorFormOwner = "#owner-toggle-select-typeahead"

	// SelectorFormContributors is the contributors multiselect
	SelectorFormContributors = "#contributors-select-toggle-select-multi-typeahead-typeahead"
)

// Form field selectors - Source Code (Git) section
const (
	// SelectorFormSourceURL is the Git repository URL input
	SelectorFormSourceURL = "input[name='source.url']"

	// SelectorFormSourceBranch is the Git branch input
	SelectorFormSourceBranch = "input[name='source.branch']"

	// SelectorFormSourcePath is the repository root path input
	SelectorFormSourcePath = "input[name='source.path']"
)

// Form field selectors - Binary (Java) section
const (
	// SelectorFormGroup is the Maven group ID input
	SelectorFormGroup = "input[name='group']"

	// SelectorFormArtifact is the Maven artifact ID input
	SelectorFormArtifact = "input[name='artifact']"

	// SelectorFormVersion is the Maven version input
	SelectorFormVersion = "input[name='version']"

	// SelectorFormPackaging is the Maven packaging type input
	SelectorFormPackaging = "input[name='packaging']"
)

// Form field selectors - Asset Repository section
const (
	// SelectorFormAssetsURL is the assets repository URL
	SelectorFormAssetsURL = "input[name='assets.url']"

	// SelectorFormAssetsBranch is the assets repository branch
	SelectorFormAssetsBranch = "input[name='assets.branch']"

	// SelectorFormAssetsPath is the assets repository path
	SelectorFormAssetsPath = "input[name='assets.path']"
)

// Form action buttons
const (
	// SelectorFormSubmit is the Create/Submit button
	SelectorFormSubmit = "button[aria-label='submit']"

	// SelectorFormCancel is the Cancel button
	SelectorFormCancel = "button[aria-label='cancel']"
)

// Analysis configuration selectors
const (
	// SelectorAnalyzeButton is the Analyze button for an application
	SelectorAnalyzeButton = "button:has-text('Analyze')"

	// SelectorAnalysisLabelSelector is the label selector input
	SelectorAnalysisLabelSelector = "input[name='labelSelector']"

	// SelectorAnalysisStartButton is the Run/Start analysis button
	SelectorAnalysisStartButton = "button:has-text('Run')"
)

// Analysis status selectors
const (
	// SelectorStatusIndicator is the analysis status text/badge
	SelectorStatusIndicator = "[data-testid='status']"
)

// Status values
const (
	// StatusCreated indicates analysis task created
	StatusCreated = "Created"

	// StatusRunning indicates analysis is running
	StatusRunning = "Running"

	// StatusSucceeded indicates analysis completed successfully
	StatusSucceeded = "Succeeded"

	// StatusFailed indicates analysis failed
	StatusFailed = "Failed"
)

// Results/Insights selectors
const (
	// SelectorResultsLink navigates to results/insights
	SelectorResultsLink = "text=Reports"

	// SelectorInsightsContainer is the main insights container
	SelectorInsightsContainer = "[data-testid='insights-table']"

	// SelectorInsightRow is a single insight row
	SelectorInsightRow = "tr[data-testid='insight-row']"

	// SelectorTagsSection is the tags section
	SelectorTagsSection = "[aria-label*='tag' i]"
)

// Alternative selectors (fallbacks)
var (
	// AlternativeApplicationsNav provides fallback selectors for navigation
	AlternativeApplicationsNav = []string{
		"a[href*='application']",
		"text=Applications",
		"[aria-label='Applications']",
		"nav >> text=Applications",
	}

	// AlternativeCreateButton provides fallback selectors for create button
	AlternativeCreateButton = []string{
		"button:has-text('Create')",
		"text=Create application",
		"[aria-label*='Create']",
		"button[data-testid='create-application']",
	}

	// AlternativeFormName provides fallback selectors for name field
	AlternativeFormName = []string{
		"input[name='name']",
		"#name",
		"input[placeholder*='name' i]",
	}

	// AlternativeFormDescription provides fallback selectors for description
	AlternativeFormDescription = []string{
		"textarea[name='description']",
		"#description",
		"textarea[placeholder*='description' i]",
	}

	// AlternativeFormSourceURL provides fallback selectors for repository URL
	AlternativeFormSourceURL = []string{
		"input[name='source.url']",
		"#sourceRepository",
		"[aria-label='source repository url']",
	}

	// AlternativeFormSourceBranch provides fallback selectors for repository branch
	AlternativeFormSourceBranch = []string{
		"input[name='source.branch']",
		"#branch",
		"[aria-label='Repository branch']",
	}

	// AlternativeFormSourcePath provides fallback selectors for repository path
	AlternativeFormSourcePath = []string{
		"input[name='source.path']",
		"#rootPath",
		"[aria-label='source repository root path']",
	}

	// AlternativeFormSubmit provides fallback selectors for submit button
	AlternativeFormSubmit = []string{
		"button[aria-label='submit']",
		"button:has-text('Create')",
		"button:has-text('Save')",
		"button[type='submit']",
	}

	// AlternativeAnalyzeButton provides fallback selectors for analyze button
	AlternativeAnalyzeButton = []string{
		"button:has-text('Analyze')",
		"[aria-label='Analyze']",
		"a:has-text('Analyze')",
	}

	// AlternativeAnalysisLabelSelector provides fallback selectors for label selector input
	AlternativeAnalysisLabelSelector = []string{
		"input[name='labelSelector']",
		"[id*='label']",
		"input[placeholder*='label' i]",
	}

	// AlternativeAnalysisStartButton provides fallback selectors for start analysis
	AlternativeAnalysisStartButton = []string{
		"button:has-text('Run')",
		"button:has-text('Start')",
		"button:has-text('Submit')",
		"button:has-text('Analyze')",
		"[aria-label*='Run']",
		"[aria-label='submit']",
		"button[type='submit']:visible",
		"footer button:has-text('Run')",
		"footer button[type='submit']",
	}

	// AlternativeStatusIndicator provides fallback selectors for status
	AlternativeStatusIndicator = []string{
		"[data-testid='status']",
		".status",
		"[aria-label*='status' i]",
		"td:has-text('Status')",
	}

	// AlternativeResultsLink provides fallback selectors for results navigation
	AlternativeResultsLink = []string{
		"text=Results",
		"text=Reports",
		"text=Insights",
		"a:has-text('Reports')",
		"[aria-label*='result' i]",
	}

	// AlternativeInsightsContainer provides fallback selectors for insights container
	AlternativeInsightsContainer = []string{
		"[data-testid='insights']",
		"[data-testid='insights-table']",
		"table[aria-label*='insight' i]",
		"#insights",
		".insights-table",
	}

	// AlternativeInsightRow provides fallback selectors for insight rows
	AlternativeInsightRow = []string{
		"tr[data-testid='insight-row']",
		"tbody tr",
		"[role='row']",
	}
)
