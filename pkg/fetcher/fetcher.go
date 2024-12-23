// Copyright Istio Authors
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

package fetcher

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/hashicorp/go-multierror"
)

// A quick list of TODOS:
// 1. Add image caching to avoid the overhead of pulling the images down everytime.
// 2. Don't create directories/files in $HOME/.triton/cache if they already exist.

type remoteImgFetcher struct{}
type tritonCacheExtractor struct{}

type imgMgr struct {
	fetcher   ImgFetcher
	extractor TritonCacheExtractor
}

// ImgFetcher fetches images from a remote registry.
type ImgFetcher interface {
	FetchImg(imgName string) (v1.Image, error)
}

// TritonCacheExtractor extracts the Triton cache from an image.
type TritonCacheExtractor interface {
	ExtractCache(img v1.Image) error
}

// ImgMgr provides high-level img processing.
type ImgMgr interface {
	FetchAndExtractCache(imgName string) error
}

// Factory function to create a new ImgMgr.
func New() ImgMgr {
	return &imgMgr{
		fetcher:   &remoteImgFetcher{},
		extractor: &tritonCacheExtractor{},
	}
}

// func saveImageToCache(path string, img v1.Image, ref name.Reference) error {
// 	out, err := os.Create(path)
// 	if err != nil {
// 		return fmt.Errorf("failed to create cache file: %w", err)
// 	}
// 	defer out.Close()

// 	err = tarball.WriteToFile(path, ref, img)
// 	if err != nil {
// 		return fmt.Errorf("failed to write image to cache: %w", err)
// 	}
// 	return nil
// }

// func loadImageFromCache(path string) (v1.Image, error) {
// 	img, err := tarball.ImageFromPath(path, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to load image from cache: %w", err)
// 	}
// 	return img, nil
// }

// FetchImg pulls the image from the registry and extracts the TritonCache
func (f *remoteImgFetcher) FetchImg(imgName string) (v1.Image, error) {
	// Parse the image name into a reference (e.g., quay.io/mtahhan/triton-cache)
	ref, err := name.ParseReference(imgName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image name: %w", err)
	}

	// Check if the image is cached locally
	// cachedImagePath := filepath.Join(os.TempDir(), "cached_images", ref.Identifier())
	// if _, err := os.Stat(cachedImagePath); err == nil {
	// 	fmt.Printf("Image %s is already cached locally.", imgName)
	// 	img, err := loadImageFromCache(cachedImagePath)
	// 	if err == nil {
	// 		return img, nil
	// 	}
	// 	fmt.Printf("Failed to load cached image, fetching from registry: %v", err)
	// }
	// Fetch the image descriptor (including the manifest)
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	// Print the image details
	fmt.Println("Img fetched successfully!!!!!!!!")

	// Get the image digest and handle the error
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}
	// Print the image digest
	fmt.Println("Img Digest:", digest)

	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}
	// Print the image size
	fmt.Printf("Img Size: %v\n", size)

	// Save the image to the cache
	// err = saveImageToCache(cachedImagePath, img, ref)
	// if err != nil {
	// 	fmt.Printf("Failed to cache image: %v", err)
	// }

	return img, nil
}

func (e *tritonCacheExtractor) ExtractCache(img v1.Image) error {
	// Handle Docker, OCI, and custom formats here.
	manifest, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}

	if manifest.MediaType == types.DockerManifestSchema2 {
		// This case, assume we have docker images with "application/vnd.docker.distribution.manifest.v2+json"
		// as the manifest media type. Note that the media type of manifest is Docker specific and
		// all OCI images would have an empty string in .MediaType field.
		_, err := extractDockerImg(img)
		if err != nil {
			return fmt.Errorf("could not extract the Triton Cache from the container image %v", err)
		}
		return nil
	}

	// We try to parse it as the "compat" variant image with a single "application/vnd.oci.image.layer.v1.tar+gzip" layer.
	_, errCompat := extractOCIStandardImg(img)
	if errCompat == nil {
		return nil
	}

	// Otherwise, we try to parse it as the *oci* variant image with custom artifact media types.
	_, errOCI := extractOCIArtifactImg(img)
	if errOCI == nil {
		return nil
	}

	// We failed to parse the image in any format, so wrap the errors and return.
	return fmt.Errorf("the given image is in invalid format as an OCI image: %v",
		multierror.Append(err,
			fmt.Errorf("could not parse as compat variant: %v", errCompat),
			fmt.Errorf("could not parse as oci variant: %v", errOCI),
		),
	)
}

func (p *imgMgr) FetchAndExtractCache(imgName string) error {
	img, err := p.fetcher.FetchImg(imgName)
	if err != nil {
		return err
	}
	return p.extractor.ExtractCache(img)
}

// extractOCIArtifactImg extracts the triton cache from the
// *oci* variant Triton Kernel Cache image:  //TODO ADD URL
func extractOCIArtifactImg(img v1.Image) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not fetch layers: %v", err)
	}

	// The image must be single-layered.
	if len(layers) != 1 {
		return nil, fmt.Errorf("number of layers must be 1 but got %d", len(layers))
	}

	// The layer type of the Triton cache itself in *oci* variant.
	const cacheLayerMediaType = "application/cache.triton.content.layer.v1+triton"

	// Find the target layer walking through the layers.
	var layer v1.Layer
	for _, l := range layers {
		mt, err := l.MediaType()
		if err != nil {
			return nil, fmt.Errorf("could not retrieve the media type: %v", err)
		}
		if mt == cacheLayerMediaType {
			layer = l
			break
		}
	}

	if layer == nil {
		return nil, fmt.Errorf("could not find the layer of type %s", cacheLayerMediaType)
	}

	// Somehow go-container registry recognizes custom artifact layers as compressed ones,
	// while the Solo's Wasm layer is actually uncompressed and therefore
	// the content itself is a raw Wasm binary. So using "Uncompressed()" here result in errors
	// since internally it tries to umcompress it as gzipped blob.
	r, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("could not get layer content: %v", err)
	}
	defer r.Close()

	// Just read it since the content is already a raw Wasm binary as mentioned above.
	ret, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not extract io.triton.cache: %v", err)
	}
	return ret, nil
}

// extractDockerImg extracts the Triton Kernel Cache from the
// *compat* variant Wasm image with the standard Docker media type: application/vnd.docker.image.rootfs.diff.tar.gzip.
// https://github.com/maryamtahhan/thunderbolt/blob/main/spec-compat.md
func extractDockerImg(img v1.Image) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not fetch layers: %v", err)
	}

	// The image must have at least one layer.
	if len(layers) == 0 {
		return nil, errors.New("number of layers must be greater than zero")
	}

	layer := layers[len(layers)-1]
	mt, err := layer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("could not get media type: %v", err)
	}

	// Media type must be application/vnd.docker.image.rootfs.diff.tar.gzip.
	if mt != types.DockerLayer {
		return nil, fmt.Errorf("invalid media type %s (expect %s)", mt, types.DockerLayer)
	}

	r, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("could not get layer content: %v", err)
	}
	defer r.Close()

	ret, err := extractTritonCacheDirectory(r)
	if err != nil {
		return nil, fmt.Errorf("could not extract Triton Kernel Cache: %v", err)
	}
	return ret, nil
}

// extractOCIStandardImg extracts the Triton Kernel Cache from the
// *compat* variant Triton Kernel image with the standard OCI media type: application/vnd.oci.image.layer.v1.tar+gzip.
// https://github.com/maryamtahhan/thunderbolt/blob/main/spec-compat.md
func extractOCIStandardImg(img v1.Image) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("could not fetch layers: %v", err)
	}

	// The image must have at least one layer.
	if len(layers) == 0 {
		return nil, fmt.Errorf("number of layers must be greater than zero")
	}

	layer := layers[len(layers)-1]
	mt, err := layer.MediaType()
	if err != nil {
		return nil, fmt.Errorf("could not get media type: %v", err)
	}

	// Check if the layer is "application/vnd.oci.image.layer.v1.tar+gzip".
	if types.OCILayer != mt {
		return nil, fmt.Errorf("invalid media type %s (expect %s)", mt, types.OCILayer)
	}

	r, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("could not get layer content: %v", err)
	}
	defer r.Close()

	ret, err := extractTritonCacheDirectory(r)
	if err != nil {
		return nil, fmt.Errorf("could not extract Triton Kernel Cache: %v", err)
	}
	return ret, nil
}

// Extracts the triton named "io.triton.cache" in a given reader for tar.gz.
// This is only used for *compat* variant.
func extractTritonCacheDirectory(r io.Reader) ([]byte, error) {
	targetDir := os.Getenv("HOME") + "/.triton/cache"
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse layer as tar.gz: %v", err)
	}

	// The target directory name to skip (but process its contents)
	const TritonCacheDirName = "io.triton.cache/"

	// Tar reader to iterate through the archive
	tr := tar.NewReader(gr)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		} else if err != nil {
			return nil, fmt.Errorf("error reading tar archive: %w", err)
		}

		// Skip directories and files that are not part of io.triton.cache
		if !strings.HasPrefix(h.Name, TritonCacheDirName) {
			continue
		}

		// Strip the prefix "io.triton.cache/" from the file path
		relativePath := strings.TrimPrefix(h.Name, TritonCacheDirName)
		if relativePath == "" {
			continue // Skip the directory itself
		}

		// Resolve the new file path under the target directory
		filePath := filepath.Join(targetDir, relativePath)

		switch h.Typeflag {
		case tar.TypeDir:
			// Create the directory in the target location
			if err := os.MkdirAll(filePath, os.FileMode(h.Mode)); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}

		case tar.TypeReg:
			// Create the file in the target location
			err := writeFile(filePath, tr, os.FileMode(h.Mode))
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s: %w", filePath, err)
			}

		default:
			// Skip unsupported types
			fmt.Printf("Skipping unsupported type: %c in file %s\n", h.Typeflag, h.Name)
		}
	}

	return nil, nil
}

// writeFile writes a file's content to disk from the tar reader
func writeFile(filePath string, tarReader io.Reader, mode os.FileMode) error {
	// Create any parent directories if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories for %s: %w", filePath, err)
	}

	// Create the file
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer outFile.Close()

	// Copy the file content
	if _, err := io.Copy(outFile, tarReader); err != nil {
		return fmt.Errorf("failed to copy content to file %s: %w", filePath, err)
	}

	// Set file permissions
	if err := os.Chmod(filePath, mode); err != nil {
		return fmt.Errorf("failed to set file permissions for %s: %w", filePath, err)
	}

	return nil
}
