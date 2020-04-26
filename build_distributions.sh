#!/bin/bash

OPERATING_SYSTEMS="linux darwin windows"
ARCHS="amd64"

TEMPLATES_DIR="../../../../cmd/poll/templates"
STATIC_DIR="../../../../cmd/poll/static"

VERSION="0.1.0"

for os in $OPERATING_SYSTEMS; do
  for arch in $ARCHS; do
    out_dir="build/${os}/${arch}/gopolls"
    echo "building ${os} / ${arch}"
    mkdir -p "$out_dir"
    env GOOS="$os" GOARCH="$arch" go build -o "$out_dir" "./cmd/poll/poll.go"
    echo "building zip file"
    # we create the zipfile inside the outdir, we also symlink the static files / templates
    (cd "$out_dir" && ln -s "$STATIC_DIR" ./static && ln -s "$TEMPLATES_DIR" ./templates)
    # now go one directory up (build/os/arch) and zip the gopolls directory
    zip_name="gopolls${VERSION}_${os}-${arch}.zip"
    (cd "build/${os}/${arch}" && zip -q -r "${zip_name}" ./gopolls)
    # move the generated zip file to out directory
    mv "build/${os}/${arch}/${zip_name}" "out/"
    # also generate hash and add it in out directory
    sha_name="gopolls${VERSION}_${os}-${arch}.sha512"
    (cd "${out_dir}" && sha512sum poll* > "${sha_name}")
    mv "${out_dir}/${sha_name}" "out/"
  done
done

echo "created files in ./out"
