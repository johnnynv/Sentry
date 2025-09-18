package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Application version information (can be overridden at build time)
var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GitBranch = "unknown"
)

// AppConfig holds application runtime configuration
type AppConfig struct {
	Action     string
	ConfigPath string
	Verbose    bool
}

// SentryApp represents the main application
type SentryApp struct {
	config         *Config
	monitorService *MonitorService
	deployService  *DeployService
	appConfig      *AppConfig
}

func main() {
	// Parse command line arguments
	appConfig := parseCommandLineArgs()

	// Setup logging
	InitializeLogger(appConfig.Verbose)

	// Print banner
	printBanner()

	// Load configuration
	config, err := LoadConfig(appConfig.ConfigPath)
	if err != nil {
		AppLogger.Fatal("Failed to load configuration: %v", err)
	}

	// Create services - order matters: deploy service first, then monitor service
	deployService := NewDeployService(config)
	monitorService := NewMonitorService(config, deployService)

	// Create application instance
	app := &SentryApp{
		config:         config,
		monitorService: monitorService,
		deployService:  deployService,
		appConfig:      appConfig,
	}

	// Execute requested action
	if err := app.executeAction(); err != nil {
		AppLogger.Fatal("Action failed: %v", err)
	}
}

// parseCommandLineArgs parses and validates command line arguments
func parseCommandLineArgs() *AppConfig {
	var appConfig AppConfig

	// Define command line flags
	flag.StringVar(&appConfig.Action, "action", "", "Action to perform: watch, trigger, validate")
	flag.StringVar(&appConfig.ConfigPath, "config", "sentry.yaml", "Path to configuration file")
	flag.BoolVar(&appConfig.Verbose, "verbose", false, "Enable verbose logging")

	// Add help flag
	showHelp := flag.Bool("help", false, "Show help information")
	showVersion := flag.Bool("version", false, "Show version information")

	flag.Parse()

	// Handle help and version flags
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		printVersionInfo()
		os.Exit(0)
	}

	// Validate required action parameter
	if appConfig.Action == "" {
		fmt.Fprintf(os.Stderr, "Error: -action parameter is required\n\n")
		printUsage()
		os.Exit(1)
	}

	// Validate action value
	validActions := []string{"watch", "trigger", "validate"}
	actionValid := false
	for _, validAction := range validActions {
		if appConfig.Action == validAction {
			actionValid = true
			break
		}
	}

	if !actionValid {
		fmt.Fprintf(os.Stderr, "Error: invalid action '%s'. Valid actions: %v\n\n", appConfig.Action, validActions)
		printUsage()
		os.Exit(1)
	}

	return &appConfig
}

// executeAction executes the requested action
func (app *SentryApp) executeAction() error {
	switch app.appConfig.Action {
	case "validate":
		return app.validateAction()
	case "trigger":
		return app.triggerAction()
	case "watch":
		return app.watchAction()
	default:
		return fmt.Errorf("unknown action: %s", app.appConfig.Action)
	}
}

// validateAction validates configuration and environment
func (app *SentryApp) validateAction() error {
	AppLogger.Info("Starting configuration and environment validation...")

	// Test repository connectivity for all configured repositories
	AppLogger.Info("Testing repository connectivity...")

	for _, repo := range app.config.Repositories {
		// Test monitor repository connectivity
		if err := app.testRepositoryConnectivity(&repo.Monitor, fmt.Sprintf("Monitor repo %s", repo.Name)); err != nil {
			return fmt.Errorf("monitor repository %s connectivity test failed: %w", repo.Name, err)
		}

		// Test deploy repository connectivity
		if err := app.testQARepositoryConnectivity(&repo.Deploy, fmt.Sprintf("Deploy repo %s", repo.Name)); err != nil {
			return fmt.Errorf("deploy repository %s connectivity test failed: %w", repo.Name, err)
		}
	}

	AppLogger.Info("All validation checks passed successfully!")
	return nil
}

// triggerAction manually triggers deployment for all configured repositories
func (app *SentryApp) triggerAction() error {
	AppLogger.Info("Starting manual deployment trigger...")

	// Group repositories by their groups
	groups := make(map[string][]string)
	individual := make([]string, 0)

	for _, repo := range app.config.Repositories {
		if repo.Group != "" {
			groups[repo.Group] = append(groups[repo.Group], repo.Name)
		} else {
			individual = append(individual, repo.Name)
		}
	}

	// Trigger group deployments
	for groupName, repoNames := range groups {
		groupConfig := app.config.Groups[groupName]
		AppLogger.InfoS("Triggering group deployment", "group", groupName, "repositories", repoNames)

		if err := app.deployService.DeployGroup(groupName, repoNames, &groupConfig); err != nil {
			return fmt.Errorf("group %s deployment failed: %w", groupName, err)
		}
	}

	// Trigger individual deployments
	for _, repoName := range individual {
		AppLogger.InfoS("Triggering individual deployment", "repo", repoName)

		// Find repo config
		var repoConfig *RepositoryConfig
		for _, repo := range app.config.Repositories {
			if repo.Name == repoName {
				repoConfig = &repo
				break
			}
		}

		if repoConfig == nil {
			return fmt.Errorf("repository configuration not found: %s", repoName)
		}

		if err := app.deployService.DeployIndividual(repoConfig); err != nil {
			return fmt.Errorf("individual deployment %s failed: %w", repoName, err)
		}
	}

	AppLogger.Info("Manual deployment trigger completed successfully!")
	return nil
}

// watchAction starts continuous monitoring of repositories
func (app *SentryApp) watchAction() error {
	AppLogger.Info("Starting continuous repository monitoring...")

	// Setup signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring in a goroutine
	monitorChan := make(chan error, 1)
	go func() {
		monitorChan <- app.startMonitoring()
	}()

	// Wait for either signal or monitor error
	select {
	case sig := <-signalChan:
		AppLogger.Info("Received signal %v, shutting down gracefully...", sig)
		return nil
	case err := <-monitorChan:
		return fmt.Errorf("monitoring failed: %w", err)
	}
}

// testRepositoryConnectivity tests if monitor repository is accessible
func (app *SentryApp) testRepositoryConnectivity(monitor *MonitorConfig, repoName string) error {
	AppLogger.Info("Testing connectivity to %s (%s)...", repoName, monitor.RepoURL)

	// Test each configured branch
	for _, branch := range monitor.Branches {
		// Try to get latest commit to test connectivity
		commit, err := app.monitorService.GetLatestCommit(monitor, branch)
		if err != nil {
			return fmt.Errorf("failed to access repository %s branch %s: %w", repoName, branch, err)
		}

		AppLogger.LogRepositoryCheck(fmt.Sprintf("%s:%s", repoName, branch), true, commit.SHA, commit.Author)
	}

	return nil
}

// testQARepositoryConnectivity tests if QA repository is accessible for deployment
func (app *SentryApp) testQARepositoryConnectivity(deploy *DeployConfig, repoName string) error {
	AppLogger.Info("Testing QA repository connectivity for %s (%s)...", repoName, deploy.QARepoURL)

	// Create a temporary monitor config for testing QA repo access
	testMonitor := &MonitorConfig{
		RepoURL:  deploy.QARepoURL,
		RepoType: deploy.RepoType,
		Auth:     deploy.Auth,
	}

	// Try to get latest commit to test connectivity
	commit, err := app.monitorService.GetLatestCommit(testMonitor, deploy.QARepoBranch)
	if err != nil {
		return fmt.Errorf("failed to access QA repository: %w", err)
	}

	AppLogger.LogRepositoryCheck(fmt.Sprintf("%s:QA", repoName), true, commit.SHA, commit.Author)
	return nil
}

// startMonitoring starts the continuous monitoring process with deployment integration
func (app *SentryApp) startMonitoring() error {
	// Create a custom monitoring loop that integrates with deployment
	AppLogger.Info("Initializing monitoring services...")

	// Perform initial repository check
	if err := app.monitorService.CheckAllRepositories(); err != nil {
		return fmt.Errorf("initial repository check failed: %w", err)
	}

	// Create monitoring loop with deployment integration
	return app.runMonitoringLoop()
}

// runMonitoringLoop runs the main monitoring loop with deployment triggers
func (app *SentryApp) runMonitoringLoop() error {
	AppLogger.Info("Starting monitoring loop (checking every %d seconds)...", app.config.PollingInterval)

	// Use the MonitorService which now includes deployment triggering
	return app.monitorService.StartMonitoring()
}

// printVersionInfo prints detailed version information
func printVersionInfo() {
	fmt.Printf("Sentry version %s\n", Version)
	fmt.Printf("Build time: %s\n", BuildTime)
	fmt.Printf("Git commit: %s\n", GitCommit)
	fmt.Printf("Git branch: %s\n", GitBranch)
}

// printBanner prints application banner
func printBanner() {
	fmt.Printf(`
╔═══════════════════════════════════════╗
║           SENTRY v%s                ║
║     Tekton Pipeline Auto-Deployer    ║
╚═══════════════════════════════════════╝

`, Version)
}

// printUsage prints command usage information
func printUsage() {
	fmt.Printf(`Usage: sentry -action=<action> [options]

Actions:
  validate    Validate configuration and environment
  trigger     Manually trigger deployment from all repositories  
  watch       Start continuous monitoring of repositories

Options:
  -config     Path to configuration file (default: sentry.yaml)
  -verbose    Enable verbose logging (default: false)
  -help       Show this help information
  -version    Show version information

Examples:
  sentry -action=validate
  sentry -action=trigger -config=my-config.yaml
  sentry -action=watch -verbose

Environment Variables:
  GITHUB_TOKEN    GitHub personal access token
  GITLAB_TOKEN    GitLab access token

For more information, see the documentation.
`)
}
