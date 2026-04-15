use std::fs;
use zed_extension_api as zed;

struct K8sCrdLspExtension {
    cached_binary_path: Option<String>,
}

impl zed::Extension for K8sCrdLspExtension {
    fn new() -> Self {
        K8sCrdLspExtension {
            cached_binary_path: None,
        }
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        if let Some(path) = worktree.which("k8s-crd-lsp") {
            return Ok(zed::Command {
                command: path,
                args: vec![],
                env: vec![],
            });
        }

        if let Some(path) = &self.cached_binary_path {
            if fs::metadata(path).map_or(false, |m| m.is_file()) {
                return Ok(zed::Command {
                    command: path.clone(),
                    args: vec![],
                    env: vec![],
                });
            }
        }

        let release = zed::latest_github_release(
            "decade-eng/k8s-crd-lsp",
            zed::GithubReleaseOptions {
                require_assets: true,
                pre_release: false,
            },
        )?;

        let (platform, arch) = zed::current_platform();
        let os_str = match platform {
            zed::Os::Mac => "darwin",
            zed::Os::Linux => "linux",
            zed::Os::Windows => return Err("Windows is not supported".into()),
        };
        let arch_str = match arch {
            zed::Architecture::Aarch64 => "arm64",
            zed::Architecture::X8664 => "amd64",
            zed::Architecture::X86 => "amd64",
        };

        let asset_name = format!("k8s-crd-lsp-{os_str}-{arch_str}.tar.gz");
        let asset = release
            .assets
            .iter()
            .find(|a| a.name == asset_name)
            .ok_or_else(|| format!("no asset found matching {asset_name}"))?;

        let version_dir = format!("k8s-crd-lsp-{}", release.version);
        let binary_path = format!("{version_dir}/k8s-crd-lsp");

        if !fs::metadata(&binary_path).map_or(false, |m| m.is_file()) {
            zed::set_language_server_installation_status(
                _language_server_id,
                &zed::LanguageServerInstallationStatus::Downloading,
            );

            zed::download_file(
                &asset.download_url,
                &version_dir,
                zed::DownloadedFileType::GzipTar,
            )
            .map_err(|e| format!("failed to download: {e}"))?;

            zed::make_file_executable(&binary_path)
                .map_err(|e| format!("failed to make executable: {e}"))?;

            if let Ok(entries) = fs::read_dir(".") {
                for entry in entries.flatten() {
                    let name = entry.file_name();
                    let name = name.to_string_lossy();
                    if name.starts_with("k8s-crd-lsp-") && name.as_ref() != version_dir {
                        let _ = fs::remove_dir_all(entry.path());
                    }
                }
            }
        }

        self.cached_binary_path = Some(binary_path.clone());

        Ok(zed::Command {
            command: binary_path,
            args: vec![],
            env: vec![],
        })
    }
}

zed::register_extension!(K8sCrdLspExtension);
