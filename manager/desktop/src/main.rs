#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

use std::cell::RefCell;
use std::rc::Rc;
use std::time::{Duration, Instant};

use i_slint_backend_winit::WinitWindowAccessor;
use slint::{Timer, TimerMode};

slint::include_modules!();

/// 小白条交互状态：
/// - 左键：交给系统原生拖动（winit drag_window，等价于 CMP WindowDraggableArea）。
/// - 右键：单击最小化，双击关闭。
struct WhiteBarState {
    last_right_down: Option<Instant>,
    minimize_timer: Timer,
}

impl WhiteBarState {
    fn new() -> Self {
        Self {
            last_right_down: None,
            minimize_timer: Timer::default(),
        }
    }
}

const DOUBLE_CLICK_MS: u64 = 300;
const SINGLE_CLICK_DELAY_MS: u64 = 350;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let ui = MainWindow::new()?;

    let theme_mode = match std::env::var("LIANT_THEME").ok().as_deref() {
        Some("dark") => 1,
        Some("light") => 2,
        _ => 0,
    };
    ui.set_theme_mode(theme_mode);

    let state = Rc::new(RefCell::new(WhiteBarState::new()));
    ui.on_bar_event({
        let state = state.clone();
        let ui = ui.clone_strong();
        move |ev: BarEvent| {
            if ev.kind != 0 {
                return;
            }

            if ev.button == 0 {
                let window = ui.window();
                let _ = window.with_winit_window(|w| {
                    let _ = w.drag_window();
                });
            } else {
                let mut st = state.borrow_mut();
                let now = Instant::now();
                let is_double = st
                    .last_right_down
                    .map_or(false, |t| now.duration_since(t) < Duration::from_millis(DOUBLE_CLICK_MS));

                if is_double {
                    st.last_right_down = None;
                    st.minimize_timer.stop();
                    let window = ui.window();
                    let _ = window.hide();
                    let _ = slint::quit_event_loop();
                } else {
                    st.last_right_down = Some(now);
                    let weak = ui.as_weak();
                    st.minimize_timer.start(
                        TimerMode::SingleShot,
                        Duration::from_millis(SINGLE_CLICK_DELAY_MS),
                        move || {
                            if let Some(ui) = weak.upgrade() {
                                ui.window().set_minimized(true);
                            }
                        },
                    );
                }
            }
        }
    });
    let _handle = ui.clone_strong();
    ui.run()?;
    Ok(())
}