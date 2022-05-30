package main

import (
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xyproto/png2svg"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UTC().UnixNano())
}

// Config contains the results of parsing the flags and arguments
type Config struct {
	inputFilename         string
	outputFilename        string
	colorOptimize         bool
	colorPink             bool
	limit                 bool
	quantize              bool
	singlePixelRectangles bool
	verbose               bool
	version               bool
}

// NewConfigFromFlags returns a Config struct, a quit message (for -v) and/or an error
func NewConfigFromFlags() (*Config, string, error) {
	var c Config

	flag.StringVar(&c.outputFilename, "o", "./", "SVG output filename")
	flag.BoolVar(&c.singlePixelRectangles, "p", false, "use only single pixel rectangles")
	flag.BoolVar(&c.colorPink, "c", false, "color expanded rectangles pink")
	flag.BoolVar(&c.verbose, "v", false, "verbose")
	flag.BoolVar(&c.version, "V", false, "version")
	flag.BoolVar(&c.limit, "l", false, "limit colors to a maximum of 4096 (#abcdef -> #ace)")
	flag.BoolVar(&c.quantize, "q", false, "deprecated (same as -l)")
	flag.BoolVar(&c.colorOptimize, "z", false, "deprecated (same as -l)")

	flag.Parse()

	if c.version {
		return nil, png2svg.VersionString, nil
	}

	c.limit = c.limit || c.quantize || c.colorOptimize

	if c.colorPink {
		c.singlePixelRectangles = false
	}

	args := flag.Args()
	if len(args) == 0 {
		return nil, "", errors.New("an input PNG filename is required")

	}
	c.inputFilename = args[0]
	c.inputFilename = strings.ReplaceAll(c.inputFilename, "\\", "/")
	c.outputFilename = strings.ReplaceAll(c.outputFilename, "\\", "/")

	return &c, "", nil
}

// Run performs the user-selected operations
func Run() error {
	var (
		box          *png2svg.Box
		x, y         int
		expanded     bool
		lastx, lasty int
		lastLine     int // one message per line / y coordinate
		done         bool
	)

	c, quitMessage, err := NewConfigFromFlags()
	if err != nil {
		return err
	} else if quitMessage != "" {
		fmt.Println(quitMessage)
		return nil
	}
	state, err := os.Stat(c.inputFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
	if state.IsDir() {
		fileList, err := GetAllFile(c.inputFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			os.Exit(1)
		}
		baseName := c.inputFilename
		for _, file := range fileList {
			fmt.Println("file: ", file)
			c.inputFilename = file
			convertOne(c, done, x, y, lastx, lasty, lastLine, box, expanded, c.outputFilename, file[len(baseName):strings.LastIndex(file, ".png")]+".svg")
		}
		return nil
	}

	return convertOne(c, done, x, y, lastx, lasty, lastLine, box, expanded, "", c.outputFilename)
}

func GetAllFile(pathname string) ([]string, error) {
	var files []string
	err := filepath.Walk(pathname, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || !strings.HasSuffix(path, ".png") {
			return nil
		}
		files = append(files, strings.ReplaceAll(path, "\\", "/"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func convertOne(c *Config, done bool, x int, y int, lastx int, lasty int, lastLine int, box *png2svg.Box, expanded bool, outputBasePath string, outputFilename string) error {
	img, err := png2svg.ReadPNG(c.inputFilename, c.verbose)
	if err != nil {
		return err
	}

	height := img.Bounds().Max.Y - img.Bounds().Min.Y

	pi := png2svg.NewPixelImage(img, c.verbose)
	pi.SetColorOptimize(c.limit)

	if c.verbose {
		fmt.Print("Placing rectangles... 0%")
	}

	percentage := 0
	lastPercentage := 0

	// Cover pixels by creating expanding rectangles, as long as there are uncovered pixels
	for !c.singlePixelRectangles && !done {

		// Select the first uncovered pixel, searching from the given coordinate
		x, y = pi.FirstUncovered(lastx, lasty)

		if c.verbose && y != lastLine {
			lastPercentage = percentage
			percentage = int((float64(y) / float64(height)) * 100.0)
			png2svg.Erase(len(fmt.Sprintf("%d%%", lastPercentage)))
			fmt.Printf("%d%%", percentage)
			lastLine = y
		}

		// Create a box at that location
		box = pi.CreateBox(x, y)
		// Expand the box to the right and downwards, until it can not expand anymore
		expanded = pi.Expand(box)

		// NOTE: Random boxes gave worse results, even though they are expanding in all directions
		// Create a random box
		//box := pi.CreateRandomBox(false)
		// Expand the box in all directions, until it can not expand anymore
		//expanded = pi.ExpandRandom(box)

		// Use the expanded box. Color pink if it is > 1x1, and colorPink is true
		pi.CoverBox(box, expanded && c.colorPink, c.limit)

		// Check if we are done, searching from the current x,y
		done = pi.Done(x, y)
	}

	if c.verbose {
		png2svg.Erase(len(fmt.Sprintf("%d%%", lastPercentage)))
		fmt.Println("100%")
	}

	if c.singlePixelRectangles {
		// Cover all remaining pixels with rectangles of size 1x1
		pi.CoverAllPixels()
	}

	// Write the SVG image to outputFilename
	filename := outputBasePath + outputFilename
	dir := filepath.Dir(filename)
	os.MkdirAll(dir, os.ModePerm)
	return pi.WriteSVG(filename)
}

func main() {
	if err := Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", strings.Title(err.Error()))
		os.Exit(1)
	}
}
