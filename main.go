package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/apoorvam/goterminal"
	"github.com/common-nighthawk/go-figure"
	"github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/objectstorage"
)

func main() {

	sourceDir := flag.String("source", "", "source directoy")
	targetBucket := flag.String("target", "", "target bucket")
	flag.Parse()

	myFigure := figure.NewFigure("staci", "shadow", true)
	myFigure.Print()
	fmt.Println("\nOCI STAtic Content Importer")

	if *sourceDir == "" || *targetBucket == "" {
		log.Fatalln("Required flags: -source -target")
	}

	osClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(common.DefaultConfigProvider())
	if err != nil {
		log.Println("Error: ", err)
		os.Exit(1)
	}

	nsRequest := objectstorage.GetNamespaceRequest{}
	nsResp, err := osClient.GetNamespace(context.Background(), nsRequest)
	if err != nil {
		log.Println("Error: ", err)
		os.Exit(1)
	}
	osNamespace := *nsResp.Value

	bucketResp, err := osClient.GetBucket(context.Background(), objectstorage.GetBucketRequest{NamespaceName: &osNamespace, BucketName: targetBucket})
	if err != nil {
		log.Println("Error: ", err)
		os.Exit(1)
	}
	bucketOCID := *bucketResp.Id

	fmt.Printf("Uploading to bucket %v with OCID %v\b", *targetBucket, bucketOCID)

	writer := goterminal.New(os.Stdout)

	bar := "."

	err = filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error listing source directory %q: %v\n", path, err)
			os.Exit(1)
		}

		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(*sourceDir, path)
		rel = filepath.ToSlash(rel)

		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		header := make([]byte, 512)
		_, err = file.Read(header)
		contentType := http.DetectContentType(header)

		// hack for css
		if strings.HasSuffix(strings.ToLower(path), ".css") {
			contentType = "text/css"
		}

		file.Seek(0, 0)

		reader := bufio.NewReader(file)

		buf := &bytes.Buffer{}

		objLen, err := io.Copy(buf, reader)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		request := objectstorage.PutObjectRequest{
			NamespaceName: nsResp.Value,
			BucketName:    targetBucket,
			ObjectName:    &rel,
			ContentLength: &objLen,
			ContentType:   &contentType,
			PutObjectBody: ioutil.NopCloser(buf),
			OpcMeta:       nil,
		}

		bar += "."
		fmt.Fprintf(writer, "Upload %q\n%v\n", rel, bar)
		writer.Print()
		_, err = osClient.PutObject(context.Background(), request)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		writer.Clear()
		return nil
	})

	writer.Reset()

	fmt.Println("\nFinished file upload.")

}
