#!/bin/bash

set -e -x

go_version=1.25.5
go_installed=false

# check goenv is installed but not init
if ! command -v goenv &> /dev/null; then
  if [ -d "$HOME/.goenv" ] ; then
    bash -c "cd $HOME/.goenv && git pull origin master"
    echo "has .goenv. try goenv init"

    export GOENV_ROOT="$HOME/.goenv"
    export PATH="$GOENV_ROOT/bin:$PATH"
    eval "$(goenv init -)"
  fi
fi


if command -v goenv &> /dev/null; then
  list=$(goenv versions)

  echo "go versions"
  echo "$list"
  
  for line in $list
  do
    if [ "$line" = "$go_version" ] ; then
      echo "$line version of go installed"
      go_installed=true
    fi
  done
  
  if [ "$go_installed" = false ] ; then
    echo "install go"
    bash -c "cd $HOME/.goenv/plugins/go-build/../.. && git checkout master && git pull && cd -"
    echo "goenv install $go_version"
    goenv install $go_version
  fi

  echo "goenv local $go_version"
  goenv local $go_version

  exit 0
fi

echo "istall go"

sudo apt-get install git -y
git clone https://github.com/go-nv/goenv.git ~/.goenv

export GOENV_ROOT="$HOME/.goenv"
export PATH="$GOENV_ROOT/bin:$PATH"
eval "$(goenv init -)"

goenv install $go_version
goenv local $go_version
