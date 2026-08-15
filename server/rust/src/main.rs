//! 배틀 레퍼리 HTTP 서비스.
//! Go 게이트웨이가 매치 생성/액션/틱을 호출하고, 결정적 상태를 JSON으로 받아간다.

mod engine;

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use axum::{
    extract::{Path, State},
    http::StatusCode,
    routing::{delete, get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};

use engine::{Match, MatchUpdate};

#[derive(Clone)]
pub struct AppState {
    matches: Arc<Mutex<HashMap<String, Match>>>,
}

#[derive(Deserialize)]
struct CreateReq {
    match_id: String,
    players: [String; 2],
}

#[derive(Deserialize)]
struct ActionReq {
    match_id: String,
    player_id: String,
    action: String,
}

#[derive(Deserialize)]
struct TickReq {
    match_id: String,
    #[serde(default = "default_dt")]
    dt_ms: i32,
}

fn default_dt() -> i32 {
    50
}

#[derive(Serialize)]
struct OkResp {
    ok: bool,
}

async fn create(State(state): State<AppState>, Json(req): Json<CreateReq>) -> Result<Json<OkResp>, (StatusCode, String)> {
    let mut matches = state.matches.lock().unwrap();
    let [p1, p2] = req.players;
    matches.insert(req.match_id.clone(), Match::new(req.match_id, p1, p2));
    Ok(Json(OkResp { ok: true }))
}

async fn action(State(state): State<AppState>, Json(req): Json<ActionReq>) -> Result<Json<MatchUpdate>, (StatusCode, String)> {
    let mut matches = state.matches.lock().unwrap();
    let m = matches
        .get_mut(&req.match_id)
        .ok_or_else(|| (StatusCode::NOT_FOUND, "match not found".into()))?;
    let events = m
        .action(&req.player_id, &req.action)
        .map_err(|e| (StatusCode::BAD_REQUEST, e))?;
    let mut update = m.update();
    update.events = events;
    Ok(Json(update))
}

async fn tick(State(state): State<AppState>, Json(req): Json<TickReq>) -> Result<Json<MatchUpdate>, (StatusCode, String)> {
    let mut matches = state.matches.lock().unwrap();
    let m = matches
        .get_mut(&req.match_id)
        .ok_or_else(|| (StatusCode::NOT_FOUND, "match not found".into()))?;
    let events = m.tick(req.dt_ms);
    let mut update = m.update();
    update.events = events;
    Ok(Json(update))
}

async fn remove(State(state): State<AppState>, Path(match_id): Path<String>) -> Json<OkResp> {
    state.matches.lock().unwrap().remove(&match_id);
    Json(OkResp { ok: true })
}

async fn health() -> Json<OkResp> {
    Json(OkResp { ok: true })
}

#[tokio::main]
async fn main() {
    // PORT가 유효한 1..65535가 아니면(예: 셸 환경의 PORT=0) 기본값 사용
    let port = std::env::var("PORT")
        .ok()
        .and_then(|p| p.parse::<u16>().ok())
        .filter(|p| *p > 0)
        .unwrap_or(8081);
    let state = AppState {
        matches: Arc::new(Mutex::new(HashMap::new())),
    };
    let app = Router::new()
        .route("/health", get(health))
        .route("/match", post(create))
        .route("/action", post(action))
        .route("/tick", post(tick))
        .route("/match/{match_id}", delete(remove))
        .with_state(state);
    let addr = format!("0.0.0.0:{port}");
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind failed");
    println!("referee listening on http://{addr}");
    axum::serve(listener, app).await.expect("server error");
}
