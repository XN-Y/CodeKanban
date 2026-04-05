package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/djherbis/times"
)

type FileInfo struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	MTime      string `json:"mtime"`
	MTimeMs    string `json:"mtime_ms"`
	MTimestamp int64  `json:"mtime_unix_ms"`
	BTime      string `json:"btime"`
	BTimeMs    string `json:"btime_ms"`
	BTimestamp int64  `json:"btime_unix_ms"`
	HasBTime   bool   `json:"has_btime"`
}

type Options struct {
	Recursive  bool
	JSONOutput bool
	SortBy     string // "mtime", "btime", "name", "size"
	Reverse    bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <directory> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  --json        Output as JSON\n")
		fmt.Fprintf(os.Stderr, "  -r            Recursive\n")
		fmt.Fprintf(os.Stderr, "  --sort=X      Sort by: mtime, btime, name, size (default: name)\n")
		fmt.Fprintf(os.Stderr, "  --reverse     Reverse sort order\n")
		os.Exit(1)
	}

	dir := os.Args[1]
	opts := parseOptions(os.Args[2:])

	files, err := collectFiles(dir, opts.Recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sortFiles(files, opts.SortBy, opts.Reverse)

	if opts.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(files)
	} else {
		printTable(files)
	}
}

func parseOptions(args []string) Options {
	opts := Options{SortBy: "name"}
	for _, arg := range args {
		switch {
		case arg == "--json":
			opts.JSONOutput = true
		case arg == "-r" || arg == "--recursive":
			opts.Recursive = true
		case arg == "--reverse":
			opts.Reverse = true
		case len(arg) > 7 && arg[:7] == "--sort=":
			opts.SortBy = arg[7:]
		}
	}
	return opts
}

func collectFiles(dir string, recursive bool) ([]*FileInfo, error) {
	var files []*FileInfo

	if recursive {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors
			}
			info, err := getFileInfo(path)
			if err == nil {
				files = append(files, info)
			}
			return nil
		})
		return files, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := getFileInfo(path)
		if err == nil {
			files = append(files, info)
		}
	}

	return files, nil
}

const timeFormatMs = "2006-01-02 15:04:05.000"

func getFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	t, err := times.Stat(path)
	if err != nil {
		return nil, err
	}

	mtime := stat.ModTime()

	info := &FileInfo{
		Path:       path,
		Name:       stat.Name(),
		Size:       stat.Size(),
		IsDir:      stat.IsDir(),
		MTime:      mtime.Format(time.RFC3339Nano),
		MTimeMs:    mtime.Format(timeFormatMs),
		MTimestamp: mtime.UnixMilli(),
		HasBTime:   t.HasBirthTime(),
	}

	if t.HasBirthTime() {
		btime := t.BirthTime()
		info.BTime = btime.Format(time.RFC3339Nano)
		info.BTimeMs = btime.Format(timeFormatMs)
		info.BTimestamp = btime.UnixMilli()
	} else {
		// Fallback to mtime if no birth time available
		info.BTime = info.MTime
		info.BTimeMs = info.MTimeMs
		info.BTimestamp = info.MTimestamp
	}

	return info, nil
}

func sortFiles(files []*FileInfo, sortBy string, reverse bool) {
	sort.Slice(files, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "mtime":
			less = files[i].MTimestamp < files[j].MTimestamp
		case "btime":
			less = files[i].BTimestamp < files[j].BTimestamp
		case "size":
			less = files[i].Size < files[j].Size
		default: // name
			less = files[i].Name < files[j].Name
		}
		if reverse {
			return !less
		}
		return less
	})
}

func printTable(files []*FileInfo) {
	fmt.Printf("%-40s %8s  %-23s  %-23s\n", "NAME", "SIZE", "MODIFIED", "BIRTH")
	fmt.Println(repeat("-", 100))

	for _, f := range files {
		name := f.Path
		if len(name) > 40 {
			name = "..." + name[len(name)-37:]
		}
		if f.IsDir {
			name += "/"
		}

		btime := f.BTimeMs
		if !f.HasBTime {
			btime = "(n/a)"
		}

		fmt.Printf("%-40s %8s  %-23s  %-23s\n",
			name,
			formatSize(f.Size),
			f.MTimeMs,
			btime,
		)
	}

	fmt.Println()
	fmt.Println("Timestamps (unix ms):")
	for _, f := range files {
		name := f.Name
		if f.IsDir {
			name += "/"
		}
		btime := fmt.Sprintf("%dms", f.BTimestamp)
		if !f.HasBTime {
			btime = "(n/a)"
		}
		fmt.Printf("  %-30s  mtime: %dms  btime: %s\n", name, f.MTimestamp, btime)
	}
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/KB)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
