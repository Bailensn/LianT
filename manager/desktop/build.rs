fn main() {
    slint_build::compile_with_config(
        "ui/app.slint",
        slint_build::CompilerConfiguration::new().with_style("cosmic".into()),
    )
    .expect("Slint UI compilation failed");
    println!("cargo:rerun-if-changed=ui/app.slint");

    // Windows: 把 LianT.ico 作为主程序图标嵌入 exe 的资源段
    #[cfg(target_os = "windows")]
    {
        let mut res = winresource::WindowsResource::new();
        res.set_icon("../resources/windows/LianT.ico");
        res.set("ProductName", "LianT");
        res.set("CompanyName", "LianT Team");
        res.set("FileDescription", "LianT Manager Desktop");
        res.set("LegalCopyright", "Apache-2.0");
        if let Err(e) = res.compile() {
            eprintln!("winresource compile failed: {e}");
            std::process::exit(1);
        }
        println!("cargo:rerun-if-changed=../resources/windows/LianT.ico");
    }
}
