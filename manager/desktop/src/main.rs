use std::cell::RefCell;
use std::rc::Rc;

slint::include_modules!();

/// Immersive title bar: left-drag moves the window (frameless).
struct TitleBarState {
    dragging: bool,
    press_phys: (i32, i32),
    press_local: (f32, f32),
}

impl TitleBarState {
    fn new() -> Self {
        Self {
            dragging: false,
            press_phys: (0, 0),
            press_local: (0.0, 0.0),
        }
    }
}

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let ui = MainWindow::new()?;

    let theme_mode = match std::env::var("LIANT_THEME").ok().as_deref() {
        Some("dark") => 1,
        Some("light") => 2,
        _ => 0,
    };
    ui.set_theme_mode(theme_mode);
    println!(
        "theme-mode = {} (0=system,1=dark,2=light)",
        theme_mode
    );

    let state = Rc::new(RefCell::new(TitleBarState::new()));
    ui.on_title_event({
        let state = state.clone();
        let ui = ui.clone_strong();
        move |ev: TitleEvent| {
            let window = ui.window();
            let mut st = state.borrow_mut();
            if ev.kind == 0 {
                let pos = window.position();
                st.dragging = true;
                st.press_phys = (pos.x, pos.y);
                st.press_local = (ev.x, ev.y);
            } else if ev.kind == 2 {
                if st.dragging && ev.button == 0 {
                    let scale = window.scale_factor();
                    let dx = ((ev.x - st.press_local.0) * scale) as i32;
                    let dy = ((ev.y - st.press_local.1) * scale) as i32;
                    window.set_position(slint::PhysicalPosition::new(
                        st.press_phys.0 + dx,
                        st.press_phys.1 + dy,
                    ));
                }
            } else if ev.kind == 1 {
                st.dragging = false;
            }
        }
    });

    ui.on_window_control({
        let ui = ui.clone_strong();
        move |ctrl: WindowCtrl| {
            let window = ui.window();
            match ctrl.cmd {
                0 => window.set_minimized(true),
                1 => {
                    let maximized = window.is_maximized();
                    window.set_maximized(!maximized);
                }
                _ => {
                    let _ = window.hide();
                    slint::quit_event_loop().ok();
                }
            }
        }
    });

    let _handle = ui.clone_strong();
    ui.run()?;
    Ok(())
}