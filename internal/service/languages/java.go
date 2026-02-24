package languages

import (
	"regexp"
	"strings"

	"github.com/felukka/koptan/internal/service/repofs"
)

// Java builds a jar with Maven or Gradle (Kotlin included) and runs it on a
// Temurin JRE.
type Java struct{}

const javaDefaultVersion = "21"

var (
	javaPomVersionRes = []*regexp.Regexp{
		regexp.MustCompile(`<maven\.compiler\.release>\s*([\d.]+)`),
		regexp.MustCompile(`<java\.version>\s*([\d.]+)`),
		regexp.MustCompile(`<maven\.compiler\.source>\s*([\d.]+)`),
		regexp.MustCompile(`<release>\s*([\d.]+)\s*</release>`),
	}
	javaGradleVersionRes = []*regexp.Regexp{
		regexp.MustCompile(`JavaLanguageVersion\.of\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`jvmToolchain\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`sourceCompatibility\s*=\s*['"]?(?:JavaVersion\.VERSION_)?([\d._]+)`),
	}
	// javaLTS are the versions with Temurin JDK and JRE images.
	javaLTS = []string{"8", "11", "17", "21", "25"}
)

func (Java) Name() string { return "java" }

func (Java) Detect(fs *repofs.FS) (*Facts, bool) {
	f := newFacts("java", javaDefaultVersion)
	f.Entrypoint = "app.jar"
	f.StartCmd = shellCmd("java $JAVA_OPTS -Dserver.port=${PORT} -jar /app/app.jar")
	switch {
	case fs.Exists("pom.xml"):
		f.PackageManager = "maven"
		f.Version = javaVersion(fs.ReadString("pom.xml"), javaPomVersionRes)
		f.Extra["builderImage"] = "maven:3.9-eclipse-temurin-" + f.Version
		f.BuildCmd = "mvn -B -q package -DskipTests"
		if fs.Exists("mvnw") {
			f.Extra["builderImage"] = "eclipse-temurin:" + f.Version + "-jdk"
			f.BuildCmd = "chmod +x mvnw && ./mvnw -B -q package -DskipTests"
		}
		f.Extra["outputDir"] = "target"
	case fs.Exists("build.gradle") || fs.Exists("build.gradle.kts"):
		f.PackageManager = "gradle"
		script := fs.ReadString("build.gradle") + fs.ReadString("build.gradle.kts")
		f.Version = javaVersion(script, javaGradleVersionRes)
		f.Extra["builderImage"] = "gradle:8-jdk" + f.Version
		f.BuildCmd = "gradle --no-daemon -q build -x test"
		if fs.Exists("gradlew") {
			f.Extra["builderImage"] = "eclipse-temurin:" + f.Version + "-jdk"
			f.BuildCmd = "chmod +x gradlew && ./gradlew --no-daemon -q build -x test"
		}
		f.Extra["outputDir"] = "build/libs"
	default:
		return f, false
	}
	return f, true
}

// javaVersion finds the Java release in a build file and rounds it up to
// an LTS release that has images; "1.8" counts as 8.
func javaVersion(doc string, res []*regexp.Regexp) string {
	for _, re := range res {
		m := re.FindStringSubmatch(doc)
		if m == nil {
			continue
		}
		v := strings.TrimPrefix(strings.ReplaceAll(m[1], "_", "."), "1.")
		return nearestLTS(major(v, javaDefaultVersion))
	}
	return javaDefaultVersion
}

func nearestLTS(v string) string {
	n := atoi(v)
	for _, lts := range javaLTS {
		if atoi(lts) >= n {
			return lts
		}
	}
	return javaLTS[len(javaLTS)-1]
}

// atoi parses a small non-negative number, returning 0 for anything else.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
