package appfactory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	koptan "github.com/felukka/koptan/api/v1alpha"
)

type JavaInfo struct {
	JavaVersion  string
	BuildTool    string
	ArtifactPath string
}

func DiscoverJava(repoDir string) (*JavaInfo, error) {
	info := &JavaInfo{}

	if _, err := os.Stat(filepath.Join(repoDir, "pom.xml")); err == nil {
		info.BuildTool = "maven"
	} else if _, err := os.Stat(filepath.Join(repoDir, "build.gradle")); err == nil {
		info.BuildTool = "gradle"
	} else if _, err := os.Stat(filepath.Join(repoDir, "build.gradle.kts")); err == nil {
		info.BuildTool = "gradle"
	} else {
		return nil, fmt.Errorf("no pom.xml or build.gradle found in %s", repoDir)
	}

	info.JavaVersion = discoverJavaVersion(repoDir, info.BuildTool)
	if info.JavaVersion == "" {
		info.JavaVersion = "21"
	}

	info.ArtifactPath = guessArtifactPath(info.BuildTool)

	return info, nil
}

func discoverJavaVersion(repoDir, buildTool string) string {
	switch buildTool {
	case "maven":
		f, err := os.Open(filepath.Join(repoDir, "pom.xml"))
		if err != nil {
			return ""
		}
		defer func() {
			_ = f.Close()
		}()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.Contains(line, "<java.version>") {
				return extractXMLValue(line, "java.version")
			}
			if strings.Contains(line, "<maven.compiler.release>") {
				return extractXMLValue(line, "maven.compiler.release")
			}
			if strings.Contains(line, "<maven.compiler.target>") {
				return extractXMLValue(line, "maven.compiler.target")
			}
		}
	case "gradle":
		for _, name := range []string{"build.gradle", "build.gradle.kts"} {
			f, err := os.Open(filepath.Join(repoDir, name))
			if err != nil {
				continue
			}

			defer func() {
				_ = f.Close()
			}()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.Contains(line, "sourceCompatibility") ||
					strings.Contains(line, "targetCompatibility") {
					for tok := range strings.FieldsSeq(line) {
						tok = strings.Trim(tok, `"'`)
						if tok == "JavaVersion.VERSION_17" || tok == "17" {
							return "17"
						}
						if tok == "JavaVersion.VERSION_21" || tok == "21" {
							return "21"
						}
						if tok == "JavaVersion.VERSION_11" || tok == "11" {
							return "11"
						}
					}
				}
			}
		}
	}
	return ""
}

func guessArtifactPath(buildTool string) string {
	switch buildTool {
	case "maven":
		return "target/*.jar"
	case "gradle":
		return "build/libs/*.jar"
	}
	return "target/*.jar"
}

func GenerateJavaApp(spec koptan.JavaAppSpec) (string, error) {
	if spec.JavaVersion == "" {
		return "", fmt.Errorf("javaVersion is required")
	}
	if spec.BuildTool == "" {
		return "", fmt.Errorf("buildTool is required")
	}
	if spec.ArtifactPath == "" {
		return "", fmt.Errorf("artifactPath is required")
	}
	if spec.BuildTool != "maven" && spec.BuildTool != "gradle" {
		return "", fmt.Errorf("buildTool must be one of: maven, gradle; got %q", spec.BuildTool)
	}

	runtimeImage := fmt.Sprintf("eclipse-temurin:%s-jre-alpine", spec.JavaVersion)

	var b strings.Builder

	switch spec.BuildTool {
	case "maven":
		fmt.Fprintf(&b, "FROM maven:3-eclipse-temurin-%s-alpine AS builder\n\n", spec.JavaVersion)
	case "gradle":
		fmt.Fprintf(&b, "FROM gradle:jdk%s-alpine AS builder\n\n", spec.JavaVersion)
	}

	if len(spec.ExtraPackages) > 0 {
		fmt.Fprintf(&b, "RUN apk add --no-cache %s\n\n", strings.Join(spec.ExtraPackages, " "))
	}

	fmt.Fprintf(&b, "WORKDIR /src\n\n")

	for k, v := range spec.Env {
		fmt.Fprintf(&b, "ENV %s=%q\n", k, v)
	}
	if len(spec.Env) > 0 {
		b.WriteString("\n")
	}

	switch spec.BuildTool {
	case "maven":
		fmt.Fprintf(&b, "COPY pom.xml .\n")
		fmt.Fprintf(&b, "RUN mvn dependency:go-offline -B -q\n\n")

		fmt.Fprintf(&b, "COPY . .\n\n")

		goal := spec.MavenGoal
		if goal == "" {
			goal = "package"
		}

		mvnParts := []string{"mvn", goal, "-B", "-DskipTests"}
		if len(spec.MavenProfiles) > 0 {
			mvnParts = append(mvnParts, "-P", strings.Join(spec.MavenProfiles, ","))
		}
		mvnParts = append(mvnParts, spec.BuildArgs...)
		fmt.Fprintf(&b, "RUN %s\n\n", strings.Join(mvnParts, " "))

	case "gradle":
		fmt.Fprintf(&b, "COPY build.gradle* settings.gradle* gradle.properties* ./\n")
		fmt.Fprintf(&b, "COPY gradle/ gradle/\n")
		fmt.Fprintf(&b, "COPY gradlew* ./\n")
		fmt.Fprintf(&b, "RUN ./gradlew dependencies --no-daemon -q 2>/dev/null || true\n\n")

		fmt.Fprintf(&b, "COPY . .\n\n")

		task := spec.GradleTask
		if task == "" {
			task = "build"
		}

		gradleParts := []string{"./gradlew", task, "--no-daemon", "-x", "test"}
		gradleParts = append(gradleParts, spec.BuildArgs...)
		fmt.Fprintf(&b, "RUN %s\n\n", strings.Join(gradleParts, " "))
	}

	fmt.Fprintf(&b, "FROM %s\n\n", runtimeImage)

	fmt.Fprintf(&b, "RUN addgroup -S appgroup && adduser -S appuser -G appgroup\n\n")

	fmt.Fprintf(&b, "WORKDIR /app\n\n")

	fmt.Fprintf(&b, "COPY --from=builder /src/%s app.jar\n\n", spec.ArtifactPath)

	fmt.Fprintf(&b, "RUN chown -R appuser:appgroup /app\n")
	fmt.Fprintf(&b, "USER appuser\n\n")

	if spec.JVMArgs != "" {
		fmt.Fprintf(&b, "ENV JAVA_OPTS=%q\n\n", spec.JVMArgs)
		fmt.Fprintf(&b, "ENTRYPOINT [\"sh\", \"-c\", \"java $JAVA_OPTS -jar app.jar\"]\n")
	} else {
		fmt.Fprintf(&b, "ENTRYPOINT [\"java\", \"-jar\", \"app.jar\"]\n")
	}

	return b.String(), nil
}
