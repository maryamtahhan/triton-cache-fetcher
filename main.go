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
		Use:   "triton-cache-fetcher",
		Short: "A tool to manage OCI images",
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
	rootCmd.Flags().BoolVarP(&createFlag, "create", "c",false, "Create an OCI image")
	rootCmd.Flags().BoolVarP(&extractFlag, "extract", "e",false, "Extract an OCI image")

	// Mark the image flag as required
	rootCmd.MarkFlagRequired("image")

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}
