{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }: {
    devShells.x86_64-darwin.default =
      let pkgs = nixpkgs.legacyPackages.x86_64-darwin;
      in pkgs.mkShell {
        buildInputs = with pkgs; [
          go
          gopls
          golangci-lint
          delve
          just
          entr
        ];
      };
  };
}
