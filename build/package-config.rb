require "yaml"
require "json"

mode, output = ARGV
abort "usage: package-config.rb baseline|current OUTPUT" unless %w[baseline current].include?(mode) && output
config = YAML.load_file(".goreleaser.yaml", aliases: true)
config["dist"] = "dist/#{mode}"
commit_date = IO.popen(["git", "show", "-s", "--format=%cI", "HEAD"], &:read).strip
config.fetch("builds").each do |build|
  build["ldflags"] = build.fetch("ldflags").map { |flag| flag.gsub("{{.Date}}", commit_date) }
end
if mode == "baseline"
  config["builds"] = [config.fetch("builds").first]
  config.fetch("builds").first["goos"] = ["linux", "darwin", "windows"]
  config.fetch("builds").each do |build|
    build.fetch("flags").delete("-trimpath")
  end
  config.fetch("universal_binaries").each do |binary|
    binary["ids"] = ["jobs-cli"]
    binary["replace"] = true
  end
  archive = config.fetch("archives").first
  archive.delete("ids")
  archive["id"] = "default"
  config["archives"] = [archive]
  config.fetch("homebrew_casks").each { |cask| cask["ids"] = ["default"] }
end
File.write(output, JSON.pretty_generate(config))
