package pyactr

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"gitlab.com/asmaloney/gactar/actr"
	"gitlab.com/asmaloney/gactar/amod"
	"gitlab.com/asmaloney/gactar/framework"
)

type PyACTR struct {
	framework.WriterHelper
	model     *actr.Model
	className string
	tmpPath   string
}

// New simply creates a new PyACTR instance and sets the tmp path.
func New(cli *cli.Context) (p *PyACTR, err error) {

	p = &PyACTR{tmpPath: "tmp"}

	return
}

func (p *PyACTR) Initialize() (err error) {
	_, err = framework.CheckForExecutable("python3")
	if err != nil {
		return
	}

	framework.IdentifyYourself("pyactr", "python3")

	err = framework.PythonCheckForPackage("pyactr")
	if err != nil {
		return
	}

	err = os.MkdirAll(p.tmpPath, os.ModePerm)
	if err != nil {
		return
	}

	return
}

func (p *PyACTR) SetModel(model *actr.Model) (err error) {
	if model.Name == "" {
		err = fmt.Errorf("model is missing name")
		return
	}

	p.model = model
	p.className = fmt.Sprintf("gactar_pyactr_%s", strings.Title(p.model.Name))

	return
}

func (p *PyACTR) Run(initialGoal string) (generatedCode, output []byte, err error) {
	outputFile, err := p.WriteModel(p.tmpPath, initialGoal)
	if err != nil {
		return
	}

	// run it!
	cmd := exec.Command("python3", outputFile)

	output, err = cmd.CombinedOutput()
	output = removeWarning(output)
	if err != nil {
		err = fmt.Errorf("%s", string(output))
		return
	}

	generatedCode = p.GetContents()

	return
}

func (p *PyACTR) WriteModel(path, initialGoal string) (outputFileName string, err error) {
	goal, err := amod.ParseChunk(p.model, initialGoal)
	if err != nil {
		err = fmt.Errorf("error in initial goal - %s", err)
		return
	}

	outputFileName = fmt.Sprintf("%s.py", p.className)
	if path != "" {
		outputFileName = fmt.Sprintf("%s/%s", path, outputFileName)
	}

	err = p.InitWriterHelper(outputFileName)
	if err != nil {
		return
	}
	defer p.CloseWriterHelper()

	p.Writeln("# This file is generated by gactar %s", time.Now().Format("2006-01-02 15:04:05"))
	p.Writeln("# https://github.com/asmaloney/gactar")
	p.Writeln("")
	p.Writeln("# *** This is a generated file. Any changes may be overwritten.")
	p.Writeln("")
	p.Write("# %s\n\n", p.model.Description)

	p.Write("import pyactr as actr\n\n")

	memory := p.model.Memory
	additionalInit := []string{"subsymbolic=True"}

	if memory.Latency != nil {
		additionalInit = append(additionalInit, fmt.Sprintf("latency_factor=%s", framework.Float64Str(*memory.Latency)))
	}

	if memory.Threshold != nil {
		additionalInit = append(additionalInit, fmt.Sprintf("retrieval_threshold=%s", framework.Float64Str(*memory.Threshold)))
	}

	p.Write("%s = actr.ACTRModel(%s)\n\n", p.className, strings.Join(additionalInit, ", "))

	// chunks
	for _, chunk := range p.model.Chunks {
		if chunk.IsInternal() {
			continue
		}

		p.Writeln("actr.chunktype('%s', '%s')", chunk.Name, strings.Join(chunk.SlotNames, ", "))
	}
	p.Writeln("")

	p.Writeln("dm = %s.decmem", p.className)
	p.Writeln("goal = %s.set_goal('goal')", p.className)
	p.Writeln("")

	if goal != nil {
		// add our goal...
		p.Writeln("initial_goal = actr.chunkstring(string='''")
		p.outputPattern(goal, 1)
		p.Writeln("''')")

		p.Writeln("goal.add(initial_goal)")
		p.Writeln("")
	}

	if p.model.HasImaginal {
		imaginal := p.model.GetImaginal()
		p.Writeln(`imaginal = %s.set_goal(name="imaginal", delay=%s)`, p.className, framework.Float64Str(imaginal.Delay))
		p.Writeln("")
	}

	// initialize
	for _, init := range p.model.Initializers {
		initializer := "dm"
		if init.Buffer != nil {
			initializer = init.Buffer.GetName()

			// allow the user-set goal to override the initializer
			if initializer == "goal" && (goal != nil) {
				continue
			}
		}
		p.Writeln("%s.add(actr.chunkstring(string='''", initializer)
		p.outputPattern(init.Pattern, 1)
		p.Writeln("'''))")
	}

	p.Writeln("")

	// productions
	for _, production := range p.model.Productions {
		if production.Description != nil {
			p.Writeln("# %s", *production.Description)
		}

		p.Writeln("%s.productionstring(name='%s', string='''", p.className, production.Name)
		for _, match := range production.Matches {
			p.outputMatch(match)
		}

		p.Writeln("\t==>")

		if production.DoStatements != nil {
			for _, statement := range production.DoStatements {
				p.outputStatement(statement)
			}
		}

		p.Write("''')\n\n")
	}

	p.Writeln("")

	// ...add our code to run
	p.Writeln("if __name__ == '__main__':")
	p.Writeln("\tsim = %s.simulation()", p.className)
	p.Writeln("\tsim.run()")
	// TODO: Add some intelligent output when logging level is info or detail
	p.Writeln("\tif goal.test_buffer('full') == True:")
	p.Writeln("\t\tprint( 'final goal: ' + str(goal.pop()) )")

	return
}

func (p *PyACTR) outputPattern(pattern *actr.Pattern, tabs int) {
	tabbedItems := framework.KeyValueList{}
	tabbedItems.Add("isa", pattern.Chunk.Name)

	for i, slot := range pattern.Slots {
		slotName := pattern.Chunk.SlotNames[i]
		addPatternSlot(&tabbedItems, slotName, slot)
	}

	p.TabWrite(tabs, tabbedItems)
}

func (p *PyACTR) outputMatch(match *actr.Match) {
	if match.Buffer != nil {
		bufferName := match.Buffer.GetName()
		chunkName := match.Pattern.Chunk.Name

		if actr.IsInternalChunkName(chunkName) {
			if chunkName == "_status" {
				status := match.Pattern.Slots[0]
				p.Writeln("\t?%s>", bufferName)
				p.Writeln("\t\tbuffer %s", status)
			}
		} else {
			p.Writeln("\t=%s>", bufferName)
			p.outputPattern(match.Pattern, 2)
		}
	} else if match.Memory != nil {
		bufferName := "retrieval"

		chunkName := match.Pattern.Chunk.Name
		if actr.IsInternalChunkName(chunkName) {
			if chunkName == "_status" {
				status := match.Pattern.Slots[0]
				p.Writeln("\t?%s>", bufferName)
				p.Writeln("\t\tstate %s", status)
			}
		} else {
			p.Writeln("\t=%s>", bufferName)
			p.Writeln("\t\tisa\t%s", chunkName)
		}
	}
}

func addPatternSlot(tabbedItems *framework.KeyValueList, slotName string, patternSlot *actr.PatternSlot) {
	for _, item := range patternSlot.Items {
		var value string
		if item.Negated {
			value = "~"
		}

		if item.Nil {
			value += "nil"
		} else if item.ID != nil {
			value += fmt.Sprintf(`"%s"`, *item.ID)
		} else if item.Num != nil {
			value += *item.Num
		} else if item.Var != nil {
			if *item.Var == "?" {
				return
			}

			value += "="
			value += strings.TrimPrefix(*item.Var, "?")
		}

		tabbedItems.Add(slotName, value)
	}
}

func (p *PyACTR) outputStatement(s *actr.Statement) {
	if s.Set != nil {
		buffer := s.Set.Buffer
		bufferName := buffer.GetName()

		p.Write("\t=%s>\n", bufferName)

		if s.Set.Slots != nil {
			tabbedItems := framework.KeyValueList{}
			tabbedItems.Add("isa", s.Set.Chunk.Name)

			for _, slot := range *s.Set.Slots {
				slotName := slot.Name

				if slot.Value.Nil {
					tabbedItems.Add(slotName, "nil")
				} else if slot.Value.Var != nil {
					tabbedItems.Add(slotName, fmt.Sprintf("=%s", *slot.Value.Var))
				} else if slot.Value.Number != nil {
					tabbedItems.Add(slotName, *slot.Value.Number)
				} else if slot.Value.Str != nil {
					tabbedItems.Add(slotName, fmt.Sprintf(`"%s"`, *slot.Value.Str))
				}
			}
			p.TabWrite(2, tabbedItems)
		} else if s.Set.Pattern != nil {
			p.outputPattern(s.Set.Pattern, 2)
		} else {
			p.Writeln("# writing text not yet handled")
		}
	} else if s.Recall != nil {
		p.Writeln("\t+retrieval>")
		p.outputPattern(s.Recall.Pattern, 2)
	} else if s.Clear != nil {
		for _, name := range s.Clear.BufferNames {
			p.Writeln("\t~%s>", name)
		}
	}
}

// removeWarning will remove the long warning whenever pyactr is run without tkinter.
func removeWarning(text []byte) []byte {
	str := string(text)

	r := regexp.MustCompile(`(?s).+warnings.warn\("Simulation GUI is set to False."\)(.+)`)
	matches := r.FindAllStringSubmatch(str, -1)
	if len(matches) == 1 {
		str = strings.TrimSpace(matches[0][1])
	}

	return []byte(str)
}
