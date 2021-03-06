#!/bin/bash
set -e
GS_PREFIX=gs://packages.kopia.io/rpm
PKGDIR=$1

if [ -z "$PKGDIR" ]; then
  echo usage $0: /path/to/dist
  exit 1
fi

if [ ! -d "$PKGDIR" ]; then
  echo $PKGDIR must be a directory containing '*.rpm' files
  exit 1
fi

architectures="x86_64 aarch64 armhfp"
distributions="unstable"

if [ "$TRAVIS_TAG" != "" ]; then
  distributions="stable testing"
fi

echo Will process distributions $distributions

WORK_DIR=/tmp/rpm-publish
#rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR"

echo Downloading packages...

for d in $distributions; do
  for a in $architectures; do
    mkdir -p $WORK_DIR/$d/$a
  done
  gsutil -m rsync -r -d $GS_PREFIX/$d $WORK_DIR/$d
done

rpm_files=$(find $1 -name '*.rpm')

# sort all files into appropriate binary directories
for f in $rpm_files; do
  bn=$(basename $f)
  if [[ "$bn" =~ ^([^0-9]+)(.*)\.([^\.]+).rpm$ ]]; then
    ver=${BASH_REMATCH[2]}
    arch=${BASH_REMATCH[3]}
    dists=""

    if [[ $ver =~ "next" ]]; then
      # ignore -next versions which are from goreleaser snapshots
      continue
    fi

    # x.y.z
    if [[ $ver =~ [0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      dists="stable testing"
    fi

    # x.y.z-prerelease
    if [[ $ver =~ [0-9]+\.[0-9]+\.[0-9]+\-.*$ ]]; then
      dists="testing"
    fi

    # yyyymmdd.0.hhmmss starts with 20
    if [[ $ver =~ 20[0-9]+\.[0-9]+\.[0-9]+ ]]; then
      dists="unstable"
    fi

    echo "f: $f arch: $arch dists: $dists"

    bn=$(basename $f)
    for d in $dists; do
      packages_dir=$WORK_DIR/$d/$arch
      cp -av $f $packages_dir
      rpm --define "%_gpg_name Kopia Builder" --addsign "$packages_dir/$bn"
    done
  fi
done

# regenerate indexes
for a in $architectures; do
    for d in $distributions; do
        rm -rf $WORK_DIR/$d/$a/repomd
        docker run -it -e verbose=true -v $WORK_DIR/$d/$a:/data sark/createrepo:latest
    done
done

echo Synchronizing...

for d in $distributions; do
  gsutil -m rsync -r -d $WORK_DIR/$d $GS_PREFIX/$d
  gsutil -m setmeta -h "Cache-Control:no-cache, max-age=0" -r $GS_PREFIX/$d/{x86_64,aarch64,armhfp}/repodata/
done

echo Done.
