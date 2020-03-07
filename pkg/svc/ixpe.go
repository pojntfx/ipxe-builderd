package svc

//go:generate mkdir -p ../proto/generated
//go:generate sh -c "protoc --go_out=paths=source_relative,plugins=grpc:../proto/generated -I=../proto ../proto/*.proto"

import (
	"errors"
	"strings"

	"github.com/minio/minio-go/v6"
	iPXEBuilder "github.com/pojntfx/ipxebuilderd/pkg/proto/generated"
	"github.com/pojntfx/ipxebuilderd/pkg/workers"
	"gitlab.com/bloom42/libs/rz-go/log"
)

const (
	totalCompileSteps = 2247
)

// IPXEBuilder manages iPXEs.
type IPXEBuilder struct {
	iPXEBuilder.UnimplementedIPXEBuilderServer
	Builder  *workers.Builder
	S3Client *minio.Client
}

// Extract extracts the iPXE source code.
func (i *IPXEBuilder) Extract() error {
	return i.Builder.Extract()
}

// ConnectToS3 connects to S3.
func (i *IPXEBuilder) ConnectToS3(s3HostPort, s3AccessKey, s3SecretKey, s3bucketName string) error {
	minioClient, err := minio.New(s3HostPort, s3AccessKey, s3SecretKey, false)

	if err := minioClient.MakeBucket(s3bucketName, "us-east-1"); err != nil {
		exists, err := minioClient.BucketExists(s3bucketName)
		if err != nil && !exists {
			return err
		}
	}

	i.S3Client = minioClient

	return err
}

func (i *IPXEBuilder) upload(path string) error {
	return nil
}

// Create creates an iPXE.
func (i *IPXEBuilder) Create(req *iPXEBuilder.IPXE, srv iPXEBuilder.IPXEBuilder_CreateServer) error {
	delta := int64(totalCompileSteps)
	stdoutChan, stderrChan, doneChan, errChan := make(chan string), make(chan string), make(chan string), make(chan error)

	go i.Builder.Build(req.GetScript(), req.GetPlatform(), req.GetDriver(), req.GetExtension(), stdoutChan, stderrChan, doneChan, errChan)

	log.Info("Building iPXE")

	for {
		select {
		case <-stdoutChan:
			delta--
			if err := srv.Send(&iPXEBuilder.IPXEStatus{
				Delta: delta,
				Path:  "",
			}); err != nil {
				return err
			}
		case stderrErr := <-stderrChan:
			if !strings.Contains(stderrErr, "ar:") { // `ar` prints to `stderr` for some reason
				return errors.New(stderrErr)
			}

			delta--
			if err := srv.Send(&iPXEBuilder.IPXEStatus{
				Delta: delta,
				Path:  "",
			}); err != nil {
				return err
			}
		case outPath := <-doneChan:
			// TODO: Upload to S3, share and send the link here

			if err := srv.Send(&iPXEBuilder.IPXEStatus{
				Delta: 0,
				Path:  outPath,
			}); err != nil {
				return err
			}

			return nil
		case err := <-errChan:
			return err
		}
	}
}
