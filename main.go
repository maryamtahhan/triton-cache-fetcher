// Copyright Red Hat Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"

	"github.com/gpuman/thunderbolt/imgmgr"
	"github.com/spf13/cobra"
)

func getCacheImage(imageName string) error {
	mgr := imgmgr.New()
	return mgr.FetchAndExtractCache(imageName)
}

func createCacheImage(imageName string) error {
	// TODO
	log.Printf("Creating image: %s", imageName)
	return nil
}

func main() {
	var imageName string
	var createFlag bool
	var extractFlag bool

	// Define the CLI command using cobra
	var rootCmd = &cobra.Command{
		Use:   "thunderbolt",
		Short: "A tool to manage GPU Kernel runtime container images",
		Run: func(cmd *cobra.Command, args []string) {
			if createFlag {
				if err := createCacheImage(imageName); err != nil {
					log.Fatalf("Error creating image: %v\n", err)
				}
			}

			if extractFlag {
				if err := getCacheImage(imageName); err != nil {
					log.Fatalf("Error extracting image: %v\n", err)
				}
			}

			if !createFlag && !extractFlag {
				log.Println("No action specified. Use --create or --extract flag.")
			}
		},
	}

	// Define the flags for the command-line arguments
	rootCmd.Flags().StringVarP(&imageName, "image", "i", "", "OCI image name")
	rootCmd.Flags().BoolVarP(&createFlag, "create", "c", false, "Create an OCI image")
	rootCmd.Flags().BoolVarP(&extractFlag, "extract", "e", false, "Extract an OCI image")

	// Mark the image flag as required
	rootCmd.MarkFlagRequired("image")

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
