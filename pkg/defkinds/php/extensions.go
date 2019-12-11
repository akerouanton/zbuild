package php

import (
	"fmt"
	"sort"
	"strings"

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

var extensionsDeps = map[string]map[string]string{
	"bz2":          {"libbz2-dev": "*"},
	"curl":         {"libcurl4-openssl-dev": "*"},
	"dom":          {"libxml2-dev": "*"},
	"enchant":      {"libenchant-dev": "*"},
	"ffi":          {"libffi-dev": "*"},
	"ftp":          {"libssl-dev": "*"},
	"gd":           {"libpng-dev": "*"},
	"gd.freetype":  {"libfreetype6-dev": "*"},
	"gd.jpeg":      {"libjpeg-dev": "*"},
	"gd.webp":      {"libwebp-dev": "*"},
	"gmp":          {"libgmp-dev": "*"},
	"imap":         {"libc-client-dev": "*", "libkrb5-dev": "*"},
	"interbase":    {}, // @TODO: could not find needed dependencies
	"intl":         {"libicu-dev": "*"},
	"ldap":         {"libldap2-dev": "*"},
	"mcrypt":       {"libmcrypt-dev": "*"},
	"oci8":         {}, // @TODO
	"odbc":         {}, // @TODO
	"pdo_dblib":    {}, // @TODO
	"pdo_firebird": {}, // @TODO
	"pdo_oci":      {}, // @TODO
	"pdo_odbc":     {}, // @TODO
	"pdo_pgsql":    {"libpq-dev": "*"},
	"pdo_sqlite":   {"libsqlite3-dev": "*"},
	"pgsql":        {"libpq-dev": "*"},
	"phar":         {"libssl-dev": "*"},
	"pspell":       {"libpspell-dev": "*"},
	"readline":     {"libedit-dev": "*"},
	"recode":       {"librecode-dev": "*"},
	"simplexml":    {"libxml2-dev": "*"},
	"snmp":         {"libsnmp-dev": "*"},
	"soap":         {"libxml2-dev": "*"},
	"sodium":       {"libsodium-dev": "*"},
	"sockets":      {"libssl-dev": "*", "openssl": "*"},
	"tidy":         {"libtidy-dev": "*"},
	"wddx":         {"libxml2-dev": "*"},
	"xml":          {"libxml2-dev": "*"},
	"xmlreader":    {}, // @TODO: this extension seems broken (bad include statement)
	"xmlrpc":       {"libxml2-dev": "*"},
	"xmlwriter":    {"libxml2-dev": "*"},
	"xsl":          {"libxslt1-dev": "*"},
	"zip":          {"libzip-dev": "*", "zlib1g-dev": "*"},
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

func InstallExtensions(state llb.State, def Definition, extensions map[string]string) llb.State {
	coreExtensions := filterExtensions(extensions, isCoreExtension)
	peclExtensions := filterExtensions(extensions, isNotCoreExtension)

	cmds := []string{}
	if len(coreExtensions) > 0 {
		coreExtensionNames := getExtensionNames(coreExtensions)
		coreExtensionSpecs := getCoreExtensionSpecs(coreExtensions)
		cmds = append(cmds, configureExtensionBuilds(coreExtensionNames)...)
		cmds = append(cmds,
			"docker-php-ext-install -j\"$(nproc)\" "+strings.Join(coreExtensionSpecs, " "))
		if version.Compare(def.MajMinVersion, "7.3", ">=") {
			cmds = append(cmds, "docker-php-source delete")
		}
	}
	if len(peclExtensions) > 0 {
		peclExtensionNames := getExtensionNames(peclExtensions)
		peclExtensionSpecs := getPeclExtensionSpecs(peclExtensions)
		cmds = append(cmds,
			"curl -f -o /usr/local/sbin/notpecl https://storage.googleapis.com/notpecl/notpecl",
			"chmod +x /usr/local/sbin/notpecl",
			"notpecl install "+strings.Join(peclExtensionSpecs, " "),
			"docker-php-ext-enable "+strings.Join(peclExtensionNames, " "),
			"rm -rf /usr/local/sbin/notpecl")
	}

	extensionNames := getExtensionNames(extensions)
	exec := state.Run(
		llbutils.Shellf(strings.Join(cmds, " && ")),
		llb.WithCustomNamef("Install PHP extensions (%s)", strings.Join(extensionNames, ", ")))

	return exec.Root()
}

var extConfigureParams = map[string][]string{
	"gd.freetype": {"--with-freetype-dir"},
	"gd.jpeg":     {"--with-jpeg-dir"},
	"gd.webp":     {"--with-webp-dir"},
	"imap":        {"--with-imap-ssl", "--with-kerberos"},
}

func configureExtensionBuilds(exts []string) []string {
	configArgs := map[string]string{}
	for _, ext := range exts {
		params, ok := extConfigureParams[ext]
		if !ok {
			continue
		}

		extName := normalizeExtName(ext)
		if _, ok := configArgs[extName]; !ok {
			configArgs[extName] = ""
		}

		configArgs[extName] = configArgs[extName] + " " + strings.Join(params, " ")
	}

	cmds := []string{}
	withGD := false
	for _, ext := range exts {
		extName := normalizeExtName(ext)
		// Skip this extension if it's GD (or one if its alias) and it's
		// already been added.
		if extName == "gd" && withGD {
			continue
		}
		if extName == "gd" {
			withGD = true
		}

		args := strings.Trim(configArgs[extName], " ")
		cmd := fmt.Sprintf("docker-php-ext-configure %s %s", extName, args)
		cmds = append(cmds, cmd)
	}

	return cmds
}

func normalizeExtName(extName string) string {
	if extName == "gd.freetype" || extName == "gd.jpeg" || extName == "gd.webp" {
		return "gd"
	}
	return extName
}
