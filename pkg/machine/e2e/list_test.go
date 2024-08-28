package e2e_test

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v5/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine list", func() {

	It("list machine", func() {
		// Random names for machines to test list
		name1 := randomString()
		name2 := randomString()

		list := new(listMachine)
		firstList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.outputToStringSlice()).To(HaveLen(1)) // just the header

		// no header and no machine should be empty
		firstList, err = mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(firstList).Should(Exit(0))
		Expect(firstList.outputToStringSlice()).To(BeEmpty()) // No header with quiet

		noheaderSession, err := mb.setCmd(list.withNoHeading()).run() // noheader
		Expect(err).NotTo(HaveOccurred())
		Expect(noheaderSession).Should(Exit(0))
		Expect(noheaderSession.outputToStringSlice()).To(BeEmpty())

		// init first machine - name1
		i := new(initMachine)
		session, err := mb.setName(name1).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		list.quiet = false // turn off quiet
		secondList, err := mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(1)) // one machine and the header

		// init second machine - name2
		i = new(initMachine)
		session2, err := mb.setName(name2).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		secondList, err = mb.setCmd(list.withQuiet()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(secondList).To(Exit(0))
		Expect(secondList.outputToStringSlice()).To(HaveLen(2)) // two machines, no header

		listNames := secondList.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())
		Expect(slices.Contains(listNames, name2)).To(BeTrue())

		// Now check a bunch of format options

		// go format
		list = new(listMachine)
		listSession, err := mb.setCmd(list.withFormat("{{.Name}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToStringSlice()).To(HaveLen(2))

		listNames = listSession.outputToStringSlice()
		stripAsterisk(listNames)
		Expect(slices.Contains(listNames, name1)).To(BeTrue())

		// --format json
		list2 := new(listMachine)
		list2 = list2.withFormat("json")
		listSession2, err := mb.setCmd(list2).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession2).To(Exit(0))
		Expect(listSession2.outputToString()).To(BeValidJSON())

		var listResponse []*entities.ListReporter
		err = jsoniter.Unmarshal(listSession2.Bytes(), &listResponse)
		Expect(err).ToNot(HaveOccurred())

		// table format includes the header
		list = new(listMachine)
		listSession3, err3 := mb.setCmd(list.withFormat("table {{.Name}}")).run()
		Expect(err3).NotTo(HaveOccurred())
		Expect(listSession3).To(Exit(0))
		listNames3 := listSession3.outputToStringSlice()
		Expect(listNames3).To(HaveLen(3)) // two machines plus a header

		// list machine in machine-readable byte format
		list = new(listMachine)
		list = list.withFormat(("json"))
		listSession, err = mb.setCmd(list).run()
		Expect(err).NotTo(HaveOccurred())

		rmName2 := new(rmMachine)
		rmName2Session, err := mb.setName(name2).setCmd(rmName2.withForce()).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(rmName2Session).To(Exit(0))

		listResponse = []*entities.ListReporter{}
		err = jsoniter.Unmarshal(listSession.Bytes(), &listResponse)
		Expect(err).NotTo(HaveOccurred())
		for _, reporter := range listResponse {
			memory, err := strconv.Atoi(reporter.Memory)
			Expect(err).NotTo(HaveOccurred())
			Expect(memory).To(BeNumerically(">", 2000000000)) // 2GiB
			diskSize, err := strconv.Atoi(reporter.DiskSize)
			Expect(err).NotTo(HaveOccurred())
			Expect(diskSize).To(BeNumerically(">", 11000000000)) // 11GiB
		}

		// list machine in human readable format
		list = new(listMachine)
		listSession, err = mb.setCmd(list.withFormat("{{.Memory}} {{.DiskSize}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToString()).To(Equal("2GiB 11GiB"))
	})

	It("list machine: check if running while starting", func() {
		skipIfWSL("the below logic does not work on WSL.  #20978")
		i := new(initMachine)
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		l := new(listMachine)
		listSession, err := mb.setCmd(l.withFormat("{{.LastUp}}")).run()
		Expect(err).NotTo(HaveOccurred())
		Expect(listSession).To(Exit(0))
		Expect(listSession.outputToString()).To(Equal("Never"))

		// The logic in this test stanza is seemingly invalid on WSL.
		// issue #20978 reflects this change
		s := new(startMachine)
		startSession, err := mb.setCmd(s).runWithoutWait()
		Expect(err).ToNot(HaveOccurred())
		wait := 3
		retries := (int)(mb.timeout/time.Second) / wait
		for i := 0; i < retries; i++ {
			listSession, err := mb.setCmd(l).run()
			Expect(listSession).To(Exit(0))
			Expect(err).ToNot(HaveOccurred())
			if startSession.ExitCode() == -1 {
				Expect(listSession.outputToString()).NotTo(ContainSubstring("Currently running"))
			} else {
				break
			}
			time.Sleep(time.Duration(wait) * time.Second)
		}
		Expect(startSession).To(Exit(0))
		listSession, err = mb.setCmd(l).run()
		Expect(listSession).To(Exit(0))
		Expect(err).ToNot(HaveOccurred())
		Expect(listSession.outputToString()).To(ContainSubstring("Currently running"))
		Expect(listSession.outputToString()).NotTo(ContainSubstring("Less than a second ago")) // check to make sure time created is accurate
	})
})

func stripAsterisk(sl []string) {
	for idx, val := range sl {
		sl[idx] = strings.TrimRight(val, "*")
	}
}
