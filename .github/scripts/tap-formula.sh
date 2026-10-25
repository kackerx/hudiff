#!/bin/sh
# usage: tap-formula.sh <tag> <sha256 of the tag's source tarball>
# Prints Formula/hudiff.rb for kackerx/homebrew-tap (release.yml): built from
# the tag's source with Go. The web page and the skill are go:embed-ded, so the
# source build is the whole app.
set -eu
tag=$1 sha=$2
cat <<RUBY
class Hudiff < Formula
  desc "Diffs for humans: an agent explains a diff, you comment line by line in the browser"
  homepage "https://github.com/kackerx/hudiff"
  url "https://github.com/kackerx/hudiff/archive/refs/tags/${tag}.tar.gz"
  sha256 "${sha}"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=v#{version}"), "."
  end

  test do
    assert_equal "v#{version}", shell_output("#{bin}/hudiff version").strip
  end
end
RUBY
