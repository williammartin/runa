package integration_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Integration", func() {
	var (
		fixture string
		tempDir string
	)

	BeforeEach(func() {
		tempDir = mktmp()
		fixture = "./test_assets/itchy"
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	Describe("build", func() {
		JustBeforeEach(func() {
			runaCmd := exec.Command(runaBinPath, "build", fixture, "-t", "my/app")
			Eventually(execBin(runaCmd), "5m").Should(gexec.Exit(0))
		})

		AfterEach(func() {
			removeImageCmd := exec.Command("docker", "image", "rm", "my/app")
			Eventually(execBin(removeImageCmd)).Should(gexec.Exit(0))
		})

		Describe("with a local directory", func() {
			It("creates a docker image with the requested tag", func() {
				dockerCmd := execBin(exec.Command("docker", "image", "list"))
				Eventually(dockerCmd).Should(gbytes.Say("my/app"))
			})

			Describe("the created image", func() {
				It("is based on the requested rootfs", func() {
					combine("my/app", tempDir)
					Expect(filepath.Join(tempDir, "img", "hello")).To(BeAnExistingFile())
				})

				It("has the user's code at the requested location", func() {
					combine("my/app", tempDir)
					Expect(filepath.Join(tempDir, "img", "tmp", "app", "myfile")).To(BeAnExistingFile())
				})
			})
		})

		Describe("when the app is on GitHub", func() {
			BeforeEach(func() {
				fixture = "https://github.com/williammartin/myapp"
			})

			It("is based on the requested rootfs", func() {
				combine("my/app", tempDir)
				Expect(filepath.Join(tempDir, "img", "hello")).To(BeAnExistingFile())
			})

			It("has the user's code at the requested location", func() {
				combine("my/app", tempDir)
				Expect(filepath.Join(tempDir, "img", "tmp", "app", "myfile")).To(BeAnExistingFile())
			})
		})
	})

	Describe("Run the app", func() {
		Context("with a local app", func() {
			It("executes the command in the Appfile", func() {
				runaCmd := exec.Command(runaBinPath, "run", fixture)
				Eventually(execBin(runaCmd), "5s").Should(gbytes.Say("hello"))
			})

			Context("when there is no appfile", func() {
				PIt("runs the app using the -i and -b flags", func() {})
			})

			It("auto-builds go apps when the image is 'golang'", func() {
				runaCmd := exec.Command(runaBinPath, "run", "./test_assets/uncompiled-golang")
				Eventually(execBin(runaCmd), "5m").Should(gbytes.Say("I got compiled!"))
			})

			It("passes extra arguments to the command", func() {
				runaCmd := exec.Command(runaBinPath, "run", "./test_assets/haz_busybox", "--first-arg", "first-value")
				Eventually(execBin(runaCmd), "5s").Should(gbytes.Say("--first-arg first-value"))
			})
		})

		Context("when the app is on GitHub", func() {
			It("executes the command in the Appfile", func() {
				runaCmd := exec.Command(runaBinPath, "run", "https://github.com/williammartin/myapp")
				Eventually(execBin(runaCmd), "1m").Should(gbytes.Say("hello"))
			})
		})
	})

	Describe("Execing a command", func() {
		It("allows overriding the command", func() {
			runaCmd := exec.Command(runaBinPath, "exec", fixture, "/tmp/app/hi")
			Eventually(execBin(runaCmd), "5s").Should(gbytes.Say("hi"))
		})
	})

	Describe("Testing the app", func() {
		It("executes the test-command in the Appfile", func() {
			runaCmd := exec.Command(runaBinPath, "test", fixture)
			Eventually(execBin(runaCmd), "1m").Should(gbytes.Say("ran-tests"))
		})
	})

	Describe("init", func() {
		It("creates an appfile for the requested language", func() {
			emptyDir := mktmp()
			runaCmd := exec.Command(runaBinPath, "init", emptyDir)
			Eventually(execBin(runaCmd), "1m").Should(gexec.Exit(0))

			b, err := ioutil.ReadFile(filepath.Join(emptyDir, "Appfile"))
			Expect(err).NotTo(HaveOccurred())
			Expect(b).To(Equal([]byte(`builder-image: golang`)))
		})
	})
})

func combine(tag, dest string) {
	Eventually(execBin(exec.Command("docker", "save", tag, "-o", filepath.Join(dest, "app.tar")))).Should(gexec.Exit(0))

	tarCmd := exec.Command("tar", "xf", "app.tar")
	tarCmd.Dir = dest
	Eventually(execBin(tarCmd)).Should(gexec.Exit(0))

	s := []struct{ Layers []string }{}
	f, err := os.Open(filepath.Join(dest, "manifest.json"))
	Expect(err).NotTo(HaveOccurred())
	json.NewDecoder(f).Decode(&s)

	for _, layer := range s[0].Layers {
		d := filepath.Join(dest, "img")
		Expect(os.MkdirAll(d, 0755)).To(Succeed())
		Eventually(execBin(exec.Command("tar", "xf", filepath.Join(dest, layer), "-C", d))).Should(gexec.Exit(0))
	}
}

func mktmp() string {
	tmp, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())
	return tmp
}
