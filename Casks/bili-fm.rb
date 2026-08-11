cask "bili-fm" do
  arch arm: "apple-silicon", intel: "intel"

  version "2.0.0"
  sha256 arm:   "10e104cb81945596aa4210e132ffb989508b37e6e46fdf5216830a641a29031b",
         intel: "6ca608053230170e2c291b451355de848c779357636fc4def3c688d988612612"

  url "https://github.com/vst93/bili-fm/releases/download/#{version}/bili-FM-#{version}-macos-#{arch}.dmg"
  name "bili-fm"
  desc "Listen to Bilibili content in audio-only mode"
  homepage "https://github.com/vst93/bili-fm"

  livecheck do
    url "https://github.com/vst93/bili-fm/releases"
    regex(/v?(\d+\.\d+(\.\d+)?)/i)
  end

  app "bili-FM.app"

  zap trash: "~/Library/Application Support/bili-FM"
end
