# This file shows what the Homebrew formula will look like
# It will be created automatically by GoReleaser or manually in a homebrew-tap repo

class Wtree < Formula
  desc "Git worktree management tool with advanced UX features"
  homepage "https://github.com/awhite/wtree"
  url "https://github.com/awhite/wtree/archive/v0.1.0.tar.gz"
  sha256 "WILL_BE_AUTOMATICALLY_FILLED"
  license "MIT"
  head "https://github.com/awhite/wtree.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w")
    
    # Install shell completions
    generate_completions_from_executable(bin/"wtree", "completion")
  end

  test do
    system "#{bin}/wtree", "--version"
    system "#{bin}/wtree", "--help"
  end
end