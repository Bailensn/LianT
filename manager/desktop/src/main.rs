#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

use std::cell::RefCell;
use std::rc::Rc;
use std::time::{Duration, Instant};

use slint::{Timer, TimerMode};

slint::include_modules!();

/// 小白条状态机：
/// - 左键按住拖动窗口（无边框窗口手动移动）。
/// - 右键单击最小化，右键双击关闭。
struct WhiteBarState {
    dragging: bool,
    last_local: (f32, f32),
    last_phys: (i32, i32),
    last_right_down: Option<Instant>,
    minimize_timer: Timer,
}

impl WhiteBarState {
    fn new() -> Self {
        Self {
            dragging: false,
            last_local: (0.0, 0.0),
            last_phys: (0, 0),
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
            let window = ui.window();
            let mut st = state.borrow_mut();

            if ev.kind == 0 {
                if ev.button == 0 {
                    let pos = window.position();
                    st.dragging = true;
                    st.last_local = (ev.x, ev.y);
                    st.last_phys = (pos.x, pos.y);
                } else {
                    let now = Instant::now();
                    let is_double = st
                        .last_right_down
                        .map_or(false, |t| now.duration_since(t) < Duration::from_millis(DOUBLE_CLICK_MS));

                    if is_double {
                        st.last_right_down = None;
                        st.minimize_timer.stop();
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
            } else if ev.kind == 2 {
                if st.dragging && ev.button == 0 {
                    let scale = window.scale_factor();
                    let dx = ((ev.x - st.last_local.0) * scale) as i32;
                    let dy = ((ev.y - st.last_local.1) * scale) as i32;
                    let nx = st.last_phys.0 + dx;
                    let ny = st.last_phys.1 + dy;
                    window.set_position(slint::PhysicalPosition::new(nx, ny));
                    st.last_local = (ev.x, ev.y);
                    st.last_phys = (nx, ny);
                }
            } else if ev.kind == 1 {
                st.dragging = false;
            }
        }
    });
    let _handle = ui.clone_strong();
    ui.run()?;
    Ok(())
}