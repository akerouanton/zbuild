package php

var coreExts = map[string]bool{
	"bcmath":    true,
	"bz2":       true,
	"calendar":  true,
	"ctype":     true,
	"curl":      true,
	"dba":       true,
	"dom":       true,
	"enchant":   true,
	"exif":      true,
	"fileinfo":  true,
	"filter":    true,
	"ftp":       true,
	"gd":        true,
	"gettext":   true,
	"gmp":       true,
	"hash":      true,
	"iconv":     true,
	"imap":      true,
	"interbase": true,
	"intl":      true,
	"json":      true,
	"ldap":      true,
	"mbstring":  true,
	// @TODO: removed from php:7.2+
	"mcrypt":       true,
	"mysqli":       true,
	"oci8":         true,
	"odbc":         true,
	"opcache":      true,
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
	"recode":       true,
	"reflection":   true,
	"session":      true,
	"shmop":        true,
	"simplexml":    true,
	"snmp":         true,
	"soap":         true,
	"sockets":      true,
	"spl":          true,
	"standard":     true,
	"sysvmsg":      true,
	"sysvsem":      true,
	"sysvshm":      true,
	"tidy":         true,
	"tokenizer":    true,
	"wddx":         true,
	"xml":          true,
	"xmlreader":    true,
	"xmlrpc":       true,
	"xmlwriter":    true,
	"xsl":          true,
	"zip":          true,
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

	return names
}

func getExtensionSpecs(extensions map[string]string) []string {
	specs := []string{}

	for extName, extVersion := range extensions {
		spec := extName
		if extVersion != "" {
			spec = extName + "-" + extVersion
		}

		specs = append(specs, spec)
	}

	return specs
}
