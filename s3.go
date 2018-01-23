// Package acc implements github.com/golang/crypto/acme/autocert.Cache
// Copyright 2017 Reed O'Brien
package acc

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/crypto/acme/autocert"
)

// s3Prefix is a prefix to use when storing the cert cache in s3.
const s3Prefix = "acc-cache/"

// S3API implements the bits of aws-sdk-go `s3iface.S3API` that are needed to
// implmement `autocert.Cache`.
type S3API interface {
	GetObjectWithContext(aws.Context, *s3.GetObjectInput, ...request.Option) (*s3.GetObjectOutput, error)
	PutObjectWithContext(aws.Context, *s3.PutObjectInput, ...request.Option) (*s3.PutObjectOutput, error)
	DeleteObjectWithContext(aws.Context, *s3.DeleteObjectInput, ...request.Option) (*s3.DeleteObjectOutput, error)
}

// MustS3 constructs a new S3 cache implementation.
func MustS3(s3 S3API, bucket, prefix string) *S3 {
	if prefix == "" {
		prefix = s3Prefix
	}
	if bucket == "" {
		panic("bucket must be set")
	}
	return &S3{
		s3:     s3,
		bucket: bucket,
		prefix: prefix,
	}
}

// S3 implements autocert.Cache using an S3 bucket for persistence. The bucket
// must already exist.
type S3 struct {
	bucket string
	prefix string
	s3     S3API
}

// Get reads certificate data from the specified key.
func (s S3) Get(ctx context.Context, name string) ([]byte, error) {
	var (
		data []byte
		done = make(chan struct{})
		err  error
		goo  *s3.GetObjectOutput
	)

	go func() {
		goi := &s3.GetObjectInput{
			Bucket: &s.bucket,
			Key:    aws.String(s.prefix + name),
		}
		goo, err = s.s3.GetObjectWithContext(ctx, goi)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}

	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			if awsErr.Code() == "NoSuchKey" {
				return nil, autocert.ErrCacheMiss
			}
		}
		return nil, err
	}
	defer goo.Body.Close()

	data, err = ioutil.ReadAll(goo.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Put writes the certificate data to the specified key.
func (s S3) Put(ctx context.Context, name string, data []byte) error {
	var (
		done = make(chan struct{})
		err  error
		// poo  *s3.PutObjectOutput
	)
	go func() {
		poi := &s3.PutObjectInput{
			ACL:                  aws.String("private"),
			Bucket:               aws.String(s.bucket),
			Key:                  aws.String(s.prefix + name),
			Body:                 bytes.NewReader(data),
			ServerSideEncryption: aws.String("AES256"),
		}
		_, err = s.s3.PutObjectWithContext(ctx, poi)
		close(done)

	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	return err

}

// Delete removes the object at the specified key.
func (s S3) Delete(ctx context.Context, name string) error {
	var (
		done = make(chan struct{})
		// doo  *s3.DeleteObjectOutput
		err error
	)

	go func() {
		doi := &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s.prefix + name),
		}
		_, err = s.s3.DeleteObjectWithContext(ctx, doi)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	return err
}
