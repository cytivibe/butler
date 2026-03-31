package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func killButlerServe() {
	if runtime.GOOS == "windows" {
		// Kill ALL butler.exe processes except our own
		myPid := fmt.Sprintf("%d", os.Getpid())
		out, _ := exec.Command("wmic", "process", "where", "name='butler.exe'",
			"get", "processid", "/format:value").Output()
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ProcessId=") {
				pid := strings.TrimPrefix(line, "ProcessId=")
				if pid != "" && pid != myPid {
					exec.Command("taskkill", "/F", "/PID", pid).Run()
				}
			}
		}
		// Wait for Windows to release file locks
		time.Sleep(3 * time.Second)
	} else {
		exec.Command("pkill", "-f", "butler serve").Run()
	}
}

func cleanWindowsPath() {
	if runtime.GOOS != "windows" {
		return
	}
	installDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "butler")
	userPath, err := exec.Command("powershell", "-Command",
		"[Environment]::GetEnvironmentVariable('Path', 'User')").Output()
	if err != nil {
		return
	}
	pathStr := strings.TrimSpace(string(userPath))
	parts := strings.Split(pathStr, ";")
	var cleaned []string
	found := false
	for _, p := range parts {
		if strings.EqualFold(strings.TrimSpace(p), installDir) {
			found = true
			continue
		}
		cleaned = append(cleaned, p)
	}
	if found {
		newPath := strings.Join(cleaned, ";")
		exec.Command("powershell", "-Command",
			fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', '%s', 'User')", newPath)).Run()
		fmt.Println("  Removed butler from user PATH.")
	}
}

func runUninstall(args []string) {
	force := false
	for _, arg := range args {
		if arg == "--force" {
			force = true
		}
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine binary path: %s\n", err)
		os.Exit(1)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	// On macOS/Linux, check if we need sudo and refuse to run without it
	if runtime.GOOS != "windows" {
		dir := filepath.Dir(exe)
		testFile := filepath.Join(dir, ".butler-uninstall-test")
		f, err := os.Create(testFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "butler: permission denied. Run: sudo butler uninstall\n")
			os.Exit(1)
		}
		f.Close()
		os.Remove(testFile)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %s\n", err)
		os.Exit(1)
	}
	dataDir := filepath.Join(home, ".butler")

	fmt.Println("This will remove:")
	fmt.Printf("  Binary:  %s\n", exe)
	if _, err := os.Stat(dataDir); err == nil {
		fmt.Printf("  Data:    %s\n", dataDir)
	}

	if !force {
		fmt.Print("\nProceed? (y/N): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	// Step 1: Kill any running butler serve processes
	fmt.Println("\nStopping butler serve processes...")
	killButlerServe()
	fmt.Println("  Done.")

	// Step 2: Clean up PATH on Windows
	cleanWindowsPath()

	// Step 3: Remove data (retry — Claude Code may respawn butler serve after we kill it)
	var removeErr error
	for attempt := 0; attempt < 5; attempt++ {
		killButlerServe()
		removeErr = os.RemoveAll(dataDir)
		if removeErr == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if removeErr != nil {
		fmt.Fprintf(os.Stderr, "Error removing %s: %s\n", dataDir, removeErr)
		fmt.Fprintln(os.Stderr, "  Another process is locking the database. Close Claude Code and try again.")
		os.Exit(1)
	}

	// Step 4: Remove binary
	if runtime.GOOS == "windows" {
		// Windows can't delete a running exe — schedule deletion after exit
		script := fmt.Sprintf(`ping -n 3 127.0.0.1 >nul & del /f "%s" & rmdir /s /q "%s"`,
			exe, filepath.Dir(exe))
		cmd := exec.Command("cmd", "/c", "start", "/min", "cmd", "/c", script)
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error scheduling binary removal: %s\n", err)
			fmt.Fprintf(os.Stderr, "  Manually delete: %s\n", exe)
			os.Exit(1)
		}
	} else {
		if err := os.Remove(exe); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing %s: %s\n", exe, err)
			os.Exit(1)
		}
	}

	fmt.Println("\nButler uninstalled.")
}
