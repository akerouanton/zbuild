package php

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/NiR-/zbuild/pkg/llbutils"
	"github.com/mcuadros/go-version"
	"github.com/moby/buildkit/client/llb"
)

var coreExts = map[string]bool{
	"bcmath":       true,
	"bz2":          true,
	"calendar":     true,
	"ctype":        true,
	"curl":         true,
	"dba":          true,
	"dom":          true,
	"enchant":      true,
	"exif":         true,
	"ffi":          true,
	"fileinfo":     true,
	"filter":       true,
	"ftp":          true,
	"gd":           true,
	"gd.freetype":  true,
	"gd.jpeg":      true,
	"gd.webp":      true,
	"gettext":      true,
	"gmp":          true,
	"hash":         true,
	"iconv":        true,
	"imap":         true,
	"interbase":    true, // @TODO: removed from php7.4+?
	"intl":         true,
	"json":         true,
	"ldap":         true,
	"mbstring":     true,
	"mcrypt":       true, // @TODO: removed from php:7.2+
	"mysqli":       true,
	"oci8":         true,
	"odbc":         true,
	"opcache":      true, // @TODO: enabled by default since php7.3+
	"pcntl":        true,
	"pdo":          true,
	"pdo_dblib":    true,
	"pdo_firebird": true,
	"pdo_mysql":    true,
	"pdo_oci":      true,
	"pdo_odbc":     true,
	"pdo_pgsql":    true,
	"pdo_sqlite":   true,
	"pgsql":        true,
	"phar":         true,
	"posix":        true,
	"pspell":       true,
	"readline":     true,
	"recode":       true, // @TODO: removed from php7.4+?
	"reflection":   true,
	"session":      true,
	"shmop":        true,
	"simplexml":    true,
	"snmp":         true,
	"soap":         true,
	"sodium":       true,
	"sockets":      true,
	"spl":          true,
	"standard":     true,
	"sysvmsg":      true,
	"sysvsem":      true,
	"sysvshm":      true,
	"tidy":         true,
	"tokenizer":    true,
	"wddx":         true, // @TODO: removed from php7.4?
	"xml":          true,
	"xmlreader":    true,
	"xmlrpc":       true,
	"xmlwriter":    true,
	"xsl":          true,
	"zip":          true,
}

var extensionsDeps = map[string]map[string]map[string]string{
	// Native extensions
	"bz2": {
		"alpine": {"bzip2-dev": "*"},
		"debian": {"libbz2-dev": "*"},
	},
	"curl": { // @TODO: remove - this extension is preinstalled
		"alpine": {"curl-dev": "*"},
		"debian": {"libcurl4-openssl-dev": "*"},
	},
	"dom": { // @TODO: enabled by default?
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"enchant": {
		"alpine": {"enchant-dev": "*"},
		"debian": {"libenchant-dev": "*"},
	},
	"ffi": {
		"alpine": {"libffi-dev": "*"},
		"debian": {"libffi-dev": "*"},
	},
	"ftp": {
		"alpine": {"openssl-dev": "*"},
		"debian": {"libssl-dev": "*"},
	},
	"gd": {
		"alpine": {"libpng-dev": "*", "zlib-dev": "*"},
		"debian": {"libpng-dev": "*"},
	},
	"gd.freetype": {
		"alpine": {"freetype-dev": "*"},
		"debian": {"libfreetype6-dev": "*"},
	},
	"gd.jpeg": {
		"alpine": {"libjpeg-turbo-dev": "*"},
		"debian": {"libjpeg-dev": "*"},
	},
	"gd.webp": {
		"alpine": {"libwebp-dev": "*"},
		"debian": {"libwebp-dev": "*"},
	},
	"gmp": {
		"alpine": {"gmp-dev": "*"},
		"debian": {"libgmp-dev": "*"},
	},
	"imap": {
		"alpine": {"imap-dev": "*"},
		"debian": {"libc-client-dev": "*", "libkrb5-dev": "*"},
	},
	"interbase": { // @TODO: remove
		"alpine": {}, // @TODO: could not find needed dependencies
		"debian": {}, // @TODO: could not find needed dependencies
	},
	"intl": {
		"alpine": {"icu-dev": "*"},
		"debian": {"libicu-dev": "*"},
	},
	"ldap": {
		"alpine": {"openldap-dev": "*"},
		"debian": {"libldap2-dev": "*"},
	},
	"mcrypt": { // @TODO: removed from latest 7.4/7.3/7.2 images
		"debian": {"libmcrypt-dev": "*"},
	},
	"oci8": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"odbc": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"pdo_dblib": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"pdo_firebird": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"pdo_oci": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"pdo_odbc": {
		"alpine": {}, // @TODO
		"debian": {}, // @TODO
	},
	"pdo_pgsql": {
		"alpine": {"postgresql-dev": "*"},
		"debian": {"libpq-dev": "*"},
	},
	"pdo_sqlite": {
		"alpine": {"sqlite-dev": "*"},
		"debian": {"libsqlite3-dev": "*"},
	},
	"pgsql": {
		"alpine": {"postgresql-dev": "*"},
		"debian": {"libpq-dev": "*"},
	},
	"phar": {
		"alpine": {"openssl-dev": "*"},
		"debian": {"libssl-dev": "*"},
	},
	"pspell": {
		"alpine": {"aspell-dev": "*"},
		"debian": {"libpspell-dev": "*"},
	},
	"readline": {
		"alpine": {"libedit-dev": "*"},
		"debian": {"libedit-dev": "*"},
	},
	"recode": { // @TODO: removed from php7.4
		"alpine": {"recode-dev": "*"},
		"debian": {"librecode-dev": "*"},
	},
	"simplexml": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"snmp": {
		"alpine": {"net-snmp-dev": "*"},
		"debian": {"libsnmp-dev": "*"},
	},
	"soap": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"sodium": {
		"alpine": {"libsodium-dev": "*"},
		"debian": {"libsodium-dev": "*"},
	},
	"sockets": {
		"alpine": {"openssl-dev": "*"},
		"debian": {"libssl-dev": "*", "openssl": "*"},
	},
	"tidy": {
		"alpine": {"tidyhtml-dev": "*"},
		"debian": {"libtidy-dev": "*"},
	},
	"wddx": { // @TODO: removed from 7.4
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"xml": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"xmlreader": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"xmlrpc": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"xmlwriter": {
		"alpine": {"libxml2-dev": "*"},
		"debian": {"libxml2-dev": "*"},
	},
	"xsl": {
		"alpine": {"libxslt-dev": "*"},
		"debian": {"libxslt1-dev": "*"},
	},
	"zip": {
		"alpine": {"libzip-dev": "*"},
		"debian": {"libzip-dev": "*", "zlib1g-dev": "*"},
	},

	// PECL Extensions
	"imagick": {
		"alpine": {"imagemagick6-dev": "*"},
		"debian": {"libmagick++-6.q16-dev": "*"},
	},
	"redis": {
		// This ext needs no deps
		"alpine": {},
		"debian": {},
	},
	"memcache": {
		"alpine": {"zlib-dev": "*"},
		"debian": {"zlib1g-dev": "*"},
	},
	"memcached": {
		"alpine": {"libmemcached-dev": "*", "zlib-dev": "*"},
		"debian": {"libmemcached-dev": "*", "zlib1g-dev": "*"},
	},
	"mongodb": {
		// This ext needs no deps
		"alpine": {},
		"debian": {},
	},
	"amqp": {
		"alpine": {"rabbitmq-c-dev": "*"},
		"debian": {"librabbitmq-dev": "*"},
	},
	"couchbase": {
		"alpine": {"libcouchbase-dev": "*"},
		"debian": {}, // @TODO: no libcouchbase available
	},
	"rdkafka": {
		"alpine": {"librdkafka-dev": "*"},
		"debian": {"librdkafka-dev": "*"},
	},
	"zookeeper": {
		"alpine": {}, // libzookeeper-dev is not available on Alpine.
		"debian": {"libzookeeper-mt-dev": "*"},
	},
}

func filterExtensions(extensions map[string]string, filterFunc func(string) bool) map[string]string {
	specs := map[string]string{}

	for extName, extSpec := range extensions {
		if filterFunc(extName) {
			specs[extName] = extSpec
		}
	}

	return specs
}

func isCoreExtension(extName string) bool {
	_, ok := coreExts[extName]
	return ok
}

func isNotCoreExtension(extName string) bool {
	_, ok := coreExts[extName]
	return !ok
}

func getExtensionNames(extensions map[string]string) []string {
	names := []string{}

	for extName := range extensions {
		names = append(names, extName)
	}

	sort.Strings(names)
	return names
}

func getCoreExtensionSpecs(extensions map[string]string) []string {
	specs := []string{}

	for extName := range extensions {
		if extName == "gd.freetype" || extName == "gd.jpeg" || extName == "gd.webp" {
			continue
		}
		specs = append(specs, extName)
	}

	sort.Strings(specs)
	return specs
}

func getPeclExtensionSpecs(extensions map[string]string) []string {
	keys := make([]string, 0, len(extensions))

	for name := range extensions {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	specs := []string{}
	for _, key := range keys {
		extVersion := extensions[key]
		spec := key
		if extVersion != "" && extVersion != "*" {
			spec = key + ":" + extVersion
		}

		specs = append(specs, spec)
	}

	return specs
}

// InstallExtensions adds a step to the given LLB state to isntall PHP
// extensions for a given StageDefinition. It uses the locked extensions
// available in the StageLocks. The same state is returned if no extensions
// are locked.
//
// The set of extensions is splitted into core extensions and community
// extensions. The former are installed using docker-php-ext-install whereas
// ther latter are installed using notpecl (a replacement for pecl). It takes
// care of deleting downloaded/unpacked files after installing extensions.
func InstallExtensions(
	stageDef StageDefinition,
	state llb.State,
	buildOpts builddef.BuildOpts,
) llb.State {
	extensions := stageDef.StageLocks.Extensions
	if len(extensions) == 0 {
		return state
	}

	coreExtensions := filterExtensions(extensions, isCoreExtension)
	peclExtensions := filterExtensions(extensions, isNotCoreExtension)

	cmds := []string{}
	if len(coreExtensions) > 0 {
		coreExtensionNames := getExtensionNames(coreExtensions)
		coreExtensionSpecs := getCoreExtensionSpecs(coreExtensions)

		cmds = append(cmds, configureExtBuilds(stageDef, coreExtensionNames)...)
		cmds = append(cmds,
			"docker-php-ext-install -j\"$(nproc)\" "+strings.Join(coreExtensionSpecs, " "))

		if version.Compare(stageDef.MajMinVersion, "7.3", ">=") {
			cmds = append(cmds, "docker-php-source delete")
		}
	}
	if len(peclExtensions) > 0 {
		peclExtensionNames := getExtensionNames(peclExtensions)
		peclExtensionSpecs := getPeclExtensionSpecs(peclExtensions)

		cmds = append(cmds,
			"curl -f -o /usr/local/sbin/notpecl https://storage.googleapis.com/notpecl/notpecl",
			"chmod +x /usr/local/sbin/notpecl")

		isAlpine := stageDef.DefLocks.OSRelease.Name == "alpine"
		if isAlpine {
			cmds = append(cmds, "apk add --no-cache --virtual=.phpize $PHPIZE_DEPS")
		}

		cmds = append(cmds,
			"notpecl install "+strings.Join(peclExtensionSpecs, " "),
			"docker-php-ext-enable "+strings.Join(peclExtensionNames, " "))

		if isAlpine {
			cmds = append(cmds, "apk del .phpize")
		}
		cmds = append(cmds, "rm -rf /usr/local/sbin/notpecl")
	}

	extensionNames := getExtensionNames(extensions)
	runOpts := []llb.RunOption{
		llbutils.Shell(cmds...),
		llb.WithCustomNamef("Install PHP extensions (%s)", strings.Join(extensionNames, ", "))}

	if buildOpts.IgnoreLayerCache {
		runOpts = append(runOpts, llb.IgnoreCache)
	}

	return state.Run(runOpts...).Root()
}

// This holds the list of flags to pass to docker-php-ext-configure. Note
// that gd has multiple entries, each alias can be specified in zbuildfiles
// to enable a specific part of gd (each having a specific set of configure
// flags and system package requirements.
// The key "7.4" holds flags for php v7.4+, whereas th key "previous" holds
// flags for v7.2 an v7.3.
var extConfigureParams = map[string]map[string][]string{
	"gd.freetype": {
		"7.4":      {"--with-freetype"},
		"previous": {"--with-freetype-dir"},
	},
	"gd.jpeg": {
		"7.4":      {"--with-jpeg"},
		"previous": {"--with-jpeg-dir"},
	},
	"gd.webp": {
		"7.4":      {"--with-webp"},
		"previous": {"--with-webp-dir"},
	},
	"imap": {
		"7.4":      {"--with-imap-ssl", "--with-kerberos"},
		"previous": {"--with-imap-ssl", "--with-kerberos"},
	},
}

// configureExtBuilds returns a list of commands to execute to configure
// extension builds using docker-php-ext-configure.
func configureExtBuilds(stageDef StageDefinition, exts []string) []string {
	paramsKey := "previous"
	if version.Compare(stageDef.MajMinVersion, "v7.4", ">=") {
		paramsKey = "7.4"
	}

	// Since gd has 3 aliases, we need to build the list of flags first
	// to merge gd flags together.
	configArgs := map[string]string{}
	for _, ext := range exts {
		params, ok := extConfigureParams[ext]
		if !ok {
			continue
		}

		// The extension name is normalized such that config flags for gd are
		// all merged together.
		extName := normalizeExtName(ext)
		if _, ok := configArgs[extName]; !ok {
			configArgs[extName] = ""
		}

		flags := strings.Join(params[paramsKey], " ")
		configArgs[extName] = configArgs[extName] + " " + flags
	}

	cmds := []string{}
	for extName, flags := range configArgs {
		cmds = append(cmds, fmt.Sprintf("docker-php-ext-configure %s %s",
			extName, strings.Trim(flags, " ")))
	}

	return cmds
}

func normalizeExtName(extName string) string {
	if extName == "gd.freetype" || extName == "gd.jpeg" || extName == "gd.webp" {
		return "gd"
	}
	return extName
}
