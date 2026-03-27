use anyhow::Result;

pub struct EbpfManager;

impl EbpfManager {
    pub fn new() -> Self {
        Self {}
    }

    pub fn attach_syscall_probe(&self, program_name: &str) -> Result<bool> {
        if program_name.trim().is_empty() {
            anyhow::bail!("program_name is required");
        }
        anyhow::bail!(
            "eBPF probe attach is not wired in this build. Install and configure aya/bpftool integration."
        )
    }

    pub fn block_process(&self, pid: u32) -> Result<bool> {
        if pid == 0 {
            anyhow::bail!("pid must be > 0");
        }
        anyhow::bail!("eBPF process blocking is not wired in this build.")
    }
}
