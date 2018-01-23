package acc_test

import (
	"bytes"
	"context"
	"testing"

	"golang.org/x/crypto/acme/autocert"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/reedobrien/acc"
	"github.com/reedobrien/checkers"
)

type dummyS3API struct {
	err        error
	encryption *string
	name       string
	key        *string
	want       []byte
}

// ClosingBuffer
type ClosingBuffer struct {
	*bytes.Buffer
}

func (_ ClosingBuffer) Close() error {
	// noop as buffers are in memory
	return nil
}

func (d *dummyS3API) DeleteObjectWithContext(_ aws.Context, doi *s3.DeleteObjectInput, _ ...request.Option) (*s3.DeleteObjectOutput, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.key = doi.Key
	d.encryption = nil
	doo := &s3.DeleteObjectOutput{}
	awsutil.Copy(doo, doi)

	return doo, nil
}

func (d *dummyS3API) GetObjectWithContext(_ aws.Context, goi *s3.GetObjectInput, _ ...request.Option) (*s3.GetObjectOutput, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.key = goi.Key
	d.encryption = nil
	goo := &s3.GetObjectOutput{}
	awsutil.Copy(goo, goi)
	goo.Body = ClosingBuffer{bytes.NewBuffer(d.want)}

	return goo, nil
}

func (d *dummyS3API) PutObjectWithContext(_ aws.Context, poi *s3.PutObjectInput, _ ...request.Option) (*s3.PutObjectOutput, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.key = poi.Key
	d.encryption = poi.ServerSideEncryption

	poo := &s3.PutObjectOutput{}
	awsutil.Copy(poo, poi)

	return poo, nil
}

func newDummyS3API(name string, want []byte, err error) *dummyS3API {
	return &dummyS3API{
		err:  err,
		name: name,
		want: want,
	}
}

func TestS3CacheSuccess(t *testing.T) {
	table := []struct {
		description string
		err         error
		name        string
		prefix      string
		wantKey     string
		wantBytes   []byte
	}{
		{"test non FQDN",
			nil, "somedomain", "", "acc-cache/somedomain", []byte("certbytes")},
		{"test non FQDN with prefix",
			nil, "somedomain", "bespoke-pfx/", "bespoke-pfx/somedomain", []byte("certbytes")},
	}

	for _, test := range table {
		dummy := newDummyS3API(test.name, test.wantBytes, test.err)
		tut := acc.MustS3(dummy, "somebucket", test.prefix)

		ctx := context.Background()

		err := tut.Delete(ctx, test.name)
		checkers.OK(t, err)
		checkers.Assert(t, dummy.key != nil, "The key should have a value")
		checkers.Assert(t, dummy.encryption == nil, "This should be nil")
		checkers.Equals(t, *dummy.key, test.wantKey)

		err = tut.Put(ctx, test.name, test.wantBytes)
		checkers.OK(t, err)
		checkers.Assert(t, dummy.key != nil, "The key should have a value")
		checkers.Assert(t, dummy.encryption != nil, "This should have a value")
		checkers.Equals(t, *dummy.key, test.wantKey)
		checkers.Equals(t, *dummy.encryption, "AES256")

		got, err := tut.Get(ctx, test.name)
		checkers.OK(t, err)
		checkers.Assert(t, dummy.key != nil, "The key should have a value")
		checkers.Assert(t, dummy.encryption == nil, "This should be nil")
		checkers.Equals(t, *dummy.key, test.wantKey)
		checkers.Equals(t, got, test.wantBytes)
	}
}

func TestS3CacheErrors(t *testing.T) {
	testErr := awserr.New("NoSuchKey", "some details", nil)
	wantErr := awserr.NewRequestFailure(testErr, 404, "ASLKD")
	dummy := newDummyS3API("somedomain", []byte("unused"), wantErr)
	tut := acc.MustS3(dummy, "somebucket", "aprefix/")

	ctx := context.Background()

	err := tut.Delete(ctx, "foo")
	checkers.Equals(t, err.(awserr.Error).Code(), "NoSuchKey")

	got, err := tut.Get(ctx, "foo")
	checkers.Equals(t, err, autocert.ErrCacheMiss)
	checkers.Assert(t, got == nil, "This should be nil")

	err = tut.Put(ctx, "foo", []byte("notused"))
	checkers.Equals(t, err.(awserr.Error).Code(), "NoSuchKey")

}

func TestMustNewS3Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustNewS3 failed to panic")
		}
	}()

	dummy := newDummyS3API("panic", []byte("at the disco"), nil)
	_ = acc.MustS3(dummy, "", "")

}
