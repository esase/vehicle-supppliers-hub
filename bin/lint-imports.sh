#!/bin/bash

module="bitbucket.org/crgw/supplier-hub/"

platformFolder="internal/platform/implementations"

ROOT=$(pwd)

failed=false

for package in $platformFolder/*; do
	if [ -d "$package" ]; then
		cd $package

		# # Get the list of imports for the current package
		imports=$(go list -f '{{ join .Imports "\n" }}')

		# Loop through each import
		for importPath in $imports; do

			# Check if the import is a platform package
			if [[ $importPath == "$module$platformFolder"* && "$importPath" != "$module$package"* ]]; then
				importingPackage=$(basename "$package")
				importedPackage=$(basename "$importPath")

				failed=true

				echo "Package $module$importingPackage should not import $importedPackage"
			fi
		done

		cd $ROOT
	fi
done

if $failed ; then
	exit 1
fi
