package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
	"github.com/WenzheLiu/GoCRMS/gocrmscli"
	"log"
	"sort"
	"strconv"
	"strings"
)

func init() {
	rootCmd.AddCommand(jobsCmd)
}

type SortableJobs []*gocrmscli.Job

func (jobs SortableJobs) Swap(i, j int) {
	jobs[i], jobs[j] = jobs[j], jobs[i]
}

func (jobs SortableJobs) Len() int {
	return len(jobs)
}

func (jobs SortableJobs) Less(i, j int) bool {
	return compareStringAsInt(jobs[i].ID, jobs[j].ID) < 0
}

func compareStringAsInt(a, b string) int {
	ia, err := strconv.Atoi(a)
	if err != nil {
		return strings.Compare(a, b)
	}
	ib, err := strconv.Atoi(b)
	if err != nil {
		return strings.Compare(a, b)
	}
	return ia - ib
}

func formatCommand(command []string) string {
	// the worst case that each command contains blank so need to add two ", plus one black as separator
	n := 3 * len(command)
	for _, arg := range command {
		n += len(arg)
	}
	var cmd strings.Builder
	cmd.Grow(n)
	writeCmdArg(&cmd, command[0])
	for _, arg := range command[1:] {
		cmd.WriteString(" ")
		writeCmdArg(&cmd, arg)

	}
	return cmd.String()
}

func writeCmdArg(w *strings.Builder, arg string) {
	hasBlack := strings.Contains(arg, " ")
	if hasBlack {
		w.WriteString(`"`)
	}
	w.WriteString(arg)
	if hasBlack {
		w.WriteString(`"`)
	}
}

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Print the jobs of crms",
	Long:  `Print the jobs of crms in a table format`,
	Run: func(cmd *cobra.Command, args []string) {
		crms, err := gocrmscli.New(globalFlags.Endpoints, globalFlags.DialTimeout, globalFlags.RequestTimeout)
		defer crms.Close()
		if err != nil {
			log.Fatalln(err)
		}
		jobs, err := crms.GetJobs()
		if err != nil {
			log.Fatalln(err)
		}
		sortedJobs := make([]*gocrmscli.Job, len(jobs))
		i := 0
		for _, job := range jobs {
			sortedJobs[i] = job
			i++
		}
		sort.Sort(SortableJobs(sortedJobs))
		if isJsonKind() {
			out, err := toJson(sortedJobs)
			if err != nil {
				log.Fatalln(err)
			}
			fmt.Println(string(out))
		} else {
			for _, job := range sortedJobs {
				cmd := formatCommand(job.Command)
				fmt.Printf("%6s\t%8s\t%s\n", job.ID, job.GetStatus(), cmd)
			}
		}
	},
}
