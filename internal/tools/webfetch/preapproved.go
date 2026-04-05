// Package webfetch preapproved host list for code-related domains.
//
// For legal and security concerns, we typically only allow Web Fetch to access
// domains that the user has provided in some form. However, we make an
// exception for a list of preapproved domains that are code-related.
//
// SECURITY WARNING: These preapproved domains are ONLY for WebFetch (GET requests only).
// The sandbox system deliberately does NOT inherit this list for network restrictions,
// as arbitrary network access (POST, uploads, etc.) to these domains could enable
// data exfiltration.
package webfetch

import "strings"

// PREAPPROVED_HOSTS is the set of hostnames (and optional path-prefixed entries)
// that are auto-approved for WebFetch without requiring user permission.
// Entries with a "/" denote path-scoped approvals (e.g., "github.com/anthropics").
var PREAPPROVED_HOSTS = []string{
	// Anthropic
	"platform.claude.com",
	"code.claude.com",
	"modelcontextprotocol.io",
	"github.com/anthropics",
	"agentskills.io",

	// Top Programming Languages
	"docs.python.org",         // Python
	"en.cppreference.com",     // C/C++ reference
	"docs.oracle.com",         // Java
	"learn.microsoft.com",     // C#/.NET
	"developer.mozilla.org",   // JavaScript/Web APIs (MDN)
	"go.dev",                  // Go
	"pkg.go.dev",              // Go docs
	"www.php.net",             // PHP
	"docs.swift.org",          // Swift
	"kotlinlang.org",          // Kotlin
	"ruby-doc.org",            // Ruby
	"doc.rust-lang.org",       // Rust
	"www.typescriptlang.org",  // TypeScript

	// Web & JavaScript Frameworks/Libraries
	"react.dev",       // React
	"angular.io",      // Angular
	"vuejs.org",       // Vue.js
	"nextjs.org",      // Next.js
	"expressjs.com",   // Express.js
	"nodejs.org",      // Node.js
	"bun.sh",          // Bun
	"jquery.com",      // jQuery
	"getbootstrap.com", // Bootstrap
	"tailwindcss.com", // Tailwind CSS
	"d3js.org",        // D3.js
	"threejs.org",     // Three.js
	"redux.js.org",    // Redux
	"webpack.js.org",  // Webpack
	"jestjs.io",       // Jest
	"reactrouter.com", // React Router

	// Python Frameworks & Libraries
	"docs.djangoproject.com",    // Django
	"flask.palletsprojects.com", // Flask
	"fastapi.tiangolo.com",      // FastAPI
	"pandas.pydata.org",         // Pandas
	"numpy.org",                 // NumPy
	"www.tensorflow.org",        // TensorFlow
	"pytorch.org",               // PyTorch
	"scikit-learn.org",          // Scikit-learn
	"matplotlib.org",            // Matplotlib
	"requests.readthedocs.io",   // Requests
	"jupyter.org",               // Jupyter

	// PHP Frameworks
	"laravel.com",  // Laravel
	"symfony.com",  // Symfony
	"wordpress.org", // WordPress

	// Java Frameworks & Libraries
	"docs.spring.io",    // Spring
	"hibernate.org",     // Hibernate
	"tomcat.apache.org", // Tomcat
	"gradle.org",        // Gradle
	"maven.apache.org",  // Maven

	// .NET & C# Frameworks
	"asp.net",                // ASP.NET
	"dotnet.microsoft.com",   // .NET
	"nuget.org",              // NuGet
	"blazor.net",             // Blazor

	// Mobile Development
	"reactnative.dev",       // React Native
	"docs.flutter.dev",      // Flutter
	"developer.apple.com",   // iOS/macOS
	"developer.android.com", // Android

	// Data Science & Machine Learning
	"keras.io",        // Keras
	"spark.apache.org", // Apache Spark
	"huggingface.co",  // Hugging Face
	"www.kaggle.com",  // Kaggle

	// Databases
	"www.mongodb.com",    // MongoDB
	"redis.io",           // Redis
	"www.postgresql.org", // PostgreSQL
	"dev.mysql.com",      // MySQL
	"www.sqlite.org",     // SQLite
	"graphql.org",        // GraphQL
	"prisma.io",          // Prisma

	// Cloud & DevOps
	"docs.aws.amazon.com",  // AWS
	"cloud.google.com",     // Google Cloud
	"kubernetes.io",        // Kubernetes
	"www.docker.com",       // Docker
	"www.terraform.io",     // Terraform
	"www.ansible.com",      // Ansible
	"vercel.com/docs",      // Vercel
	"docs.netlify.com",     // Netlify
	"devcenter.heroku.com", // Heroku

	// Testing & Monitoring
	"cypress.io",    // Cypress
	"selenium.dev",  // Selenium

	// Game Development
	"docs.unity.com",        // Unity
	"docs.unrealengine.com", // Unreal Engine

	// Other Essential Tools
	"git-scm.com",    // Git
	"nginx.org",      // Nginx
	"httpd.apache.org", // Apache HTTP Server
}

// preapprovedHostnameOnly is the set of hostnames without path prefixes.
// Built at init time for O(1) lookups.
var preapprovedHostnameOnly map[string]bool

// preapprovedPathPrefixes maps hostnames to their allowed path prefixes
// for path-scoped entries (e.g., "github.com" -> ["/anthropics"]).
var preapprovedPathPrefixes map[string][]string

func init() {
	preapprovedHostnameOnly = make(map[string]bool)
	preapprovedPathPrefixes = make(map[string][]string)

	for _, entry := range PREAPPROVED_HOSTS {
		slashIdx := strings.IndexByte(entry, '/')
		if slashIdx == -1 {
			preapprovedHostnameOnly[entry] = true
		} else {
			host := entry[:slashIdx]
			path := entry[slashIdx:]
			preapprovedPathPrefixes[host] = append(preapprovedPathPrefixes[host], path)
		}
	}
}

// isPreapprovedHost checks whether the given hostname (and optional pathname)
// matches a preapproved entry. For hostname-only entries, the pathname is ignored.
// For path-scoped entries (e.g., "github.com/anthropics"), the pathname must
// match the prefix exactly or be followed by a "/" (segment boundary).
func isPreapprovedHost(hostname, pathname string) bool {
	if preapprovedHostnameOnly[hostname] {
		return true
	}

	prefixes, ok := preapprovedPathPrefixes[hostname]
	if !ok {
		return false
	}

	for _, p := range prefixes {
		// Enforce path segment boundaries: "/anthropics" must not match
		// "/anthropics-evil/malware". Only exact match or a "/" after the
		// prefix is allowed.
		if pathname == p || strings.HasPrefix(pathname, p+"/") {
			return true
		}
	}

	return false
}
