// 不弹出控制台窗口（Windows 下 GUI 应用）。
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
    press_phys: (i32, i32),
    press_local: (f32, f32),
    last_right_down: Option<Instant>,
    minimize_timer: Timer,
}

impl WhiteBarState {
    fn new() -> Self {
        Self {
            dragging: false,
            press_phys: (0, 0),
            press_local: (0.0, 0.0),
            last_right_down: None,
            minimize_timer: Timer::default(),
        }
    }
}

// 右键双击判定窗口 (ms) 与单击动作最小化延迟 (ms)。
const DOUBLE_CLICK_MS: u64 = 300;
const SINGLE_CLICK_DELAY_MS: u64 = 350;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let ui = MainWindow::new()?;

    // theme-mode: 0 = 跟随系统, 1 = 深色, 2 = 浅色（为将来手动选择预留接口）。
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
                // ---- 按下 ----
                if ev.button == 0 {
                    // 左键：记录起点，进入拖动。
                    let pos = window.position();
                    st.dragging = true;
                    st.press_phys = (pos.x, pos.y);
                    st.press_local = (ev.x, ev.y);
                } else {
                    // 右键：判定单击 / 双击。
                    let now = Instant::now();
                    let is_double = st
                        .last_right_down
                        .map_or(false, |t| now.duration_since(t) < Duration::from_millis(DOUBLE_CLICK_MS));

                    if is_double {
                        // 右键双击：关闭窗口（并取消挂起的单击最小化）。
                        st.last_right_down = None;
                        st.minimize_timer.stop();
                        let _ = window.hide();
                        let _ = slint::quit_event_loop();
                    } else {
                        // 右键单击：延时最小化（给双击留出判定窗口）。
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
                // ---- 移动：仅左键拖动时跟随指针（相对按下的偏移）----
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
                // ---- 抬起：结束拖动（无论左右键）。----
                st.dragging = false;
            }
        }
    });

    // 保持强引用，避免事件循环退出竞态。
    let _handle = ui.clone_strong();
    ui.run()?;
    Ok(())
}