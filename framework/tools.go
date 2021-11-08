package framework

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/asmaloney/gactar/actr"
	"github.com/asmaloney/gactar/amod"
)

// Some tools for working with our frameworks

// IdentifyYourself outputs version info and the path to an executable.
func IdentifyYourself(frameworkName, exeName string) {
	cmd := exec.Command(exeName, "--version")
	output, _ := cmd.CombinedOutput()

	version := strings.TrimSpace(string(output))

	cmd = exec.Command("which", exeName)
	output, _ = cmd.CombinedOutput()

	fmt.Printf("%s: Using %s from %s", frameworkName, version, string(output))
}

func ParseInitialBuffers(model *actr.Model, initialBuffers InitialBuffers) (parsed ParsedInitialBuffers, err error) {
	parsed = ParsedInitialBuffers{}

	for bufferName, bufferInit := range initialBuffers {
		buffer := model.LookupBuffer(bufferName)
		if buffer == nil {
			err = fmt.Errorf("ERROR cannot initialize buffer '%s' - not found in model '%s'", bufferName, model.Name)
			return
		}

		pattern, parseErr := amod.ParseChunk(model, bufferInit)
		if parseErr != nil {
			err = fmt.Errorf("ERROR in initial buffer  '%s' - %s", bufferName, parseErr)
			return
		}

		parsed[bufferName] = pattern
	}

	return
}

// Float64Str takes a float and returns a string of the minimal representation.
// e.g. 2.5000 becomes "2.5"
func Float64Str(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// RemoveTempFile removes the given file if it exists.
func RemoveTempFile(filePath string) error {
	_, err := os.Stat(filePath)
	if err == nil {
		return os.Remove(filePath)
	}

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// PythonCheckForPackage checks for the proper installation of the named package.
func PythonCheckForPackage(packageName string) (err error) {
	importCmd := fmt.Sprintf("import %s", packageName)

	cmd := exec.Command("python3", "-c", importCmd)

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("python package '%s' not found. Please ensure it is installed with pip or is in your PYTHONPATH env variable", packageName)
		return
	}

	return
}

func PythonValuesToStrings(values *[]*actr.Value, quoteStrings bool) []string {
	str := make([]string, len(*values))
	for i, v := range *values {
		if v.Var != nil {
			str[i] = strings.TrimPrefix(*v.Var, "?")
		} else if v.Str != nil {
			if quoteStrings {
				str[i] = fmt.Sprintf("'%s'", *v.Str)
			} else {
				str[i] = *v.Str
			}
		} else if v.Number != nil {
			str[i] = *v.Number
		}
		// v.ID should not be possible because of validation
	}

	return str
}
