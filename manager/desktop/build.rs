fn main() {
    slint_build::compile_with_config(
        "ui/app.slint",
        slint_build::CompilerConfiguration::new().with_style("cosmic".into()),
    )
    .expect("Slint UI compilation failed");
    println!("cargo:rerun-if-changed=ui/app.slint");
}