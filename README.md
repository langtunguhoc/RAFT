# RAFT Consensus Algorithm Implementation

## Triển Khai Thuật Toán Đồng Thuận RAFT trong Hệ Thống Phân Tán

---

## Mục Lục

1. [Giới Thiệu](#1-giới-thiệu)
2. [Nền Tảng Lý Thuyết](#2-nền-tảng-lý-thuyết)
3. [Mô Hình Hệ Thống](#3-mô-hình-hệ-thống)
4. [Thuật Toán RAFT Chi Tiết](#4-thuật-toán-raft-chi-tiết)
5. [Đảm Bảo Tính Đúng Đắn](#5-đảm-bảo-tính-đúng-đắn)
6. [Phân Tích Độ Phức Tạp](#6-phân-tích-độ-phức-tạp)
7. [So Sánh Với Các Thuật Toán Khác](#7-so-sánh-với-các-thuật-toán-khác)
8. [Chi Tiết Triển Khai](#8-chi-tiết-triển-khai)
9. [Thực Nghiệm và Đánh Giá](#9-thực-nghiệm-và-đánh-giá)
10. [Hướng Dẫn Sử Dụng](#10-hướng-dẫn-sử-dụng)
11. [Tài Liệu Tham Khảo](#11-tài-liệu-tham-khảo)

---

## 1. Giới Thiệu

### 1.1 Bối Cảnh và Động Lực

Trong các hệ thống phân tán, **bài toán đồng thuận (consensus problem)** là một trong những thách thức cơ bản nhất. Bài toán này yêu cầu một tập hợp các tiến trình (processes) phải thống nhất về một giá trị duy nhất, ngay cả khi một số tiến trình có thể gặp lỗi.

**Định nghĩa hình thức của bài toán đồng thuận:**

Cho tập N tiến trình P = {p₁, p₂, ..., pₙ}, mỗi tiến trình pᵢ có:
- Giá trị đề xuất (proposed value): vᵢ
- Giá trị quyết định (decided value): dᵢ

Thuật toán đồng thuận phải thỏa mãn:
1. **Agreement (Thống nhất)**: ∀pᵢ, pⱼ ∈ P: dᵢ = dⱼ
2. **Validity (Hợp lệ)**: ∀pᵢ ∈ P: dᵢ ∈ {v₁, v₂, ..., vₙ}
3. **Termination (Kết thúc)**: Mọi tiến trình không lỗi cuối cùng sẽ quyết định

### 1.2 Định Lý FLP và Giới Hạn Lý Thuyết

**Định lý Fischer-Lynch-Paterson (1985)** chứng minh rằng trong mô hình bất đồng bộ với ít nhất một tiến trình có thể lỗi, không tồn tại thuật toán đồng thuận xác định nào đảm bảo cả ba tính chất trên.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ĐỊNH LÝ FLP (1985)                               │
├─────────────────────────────────────────────────────────────────────┤
│ "Không tồn tại giao thức đồng thuận xác định trong hệ thống         │
│  bất đồng bộ có thể chịu được sự cố của dù chỉ một tiến trình"     │
│                                                                      │
│ Hệ quả: Mọi thuật toán đồng thuận thực tế phải:                     │
│   • Sử dụng randomization (như Paxos, RAFT), HOẶC                   │
│   • Sử dụng failure detectors, HOẶC                                 │
│   • Giả định partial synchrony                                      │
└─────────────────────────────────────────────────────────────────────┘
```

RAFT sử dụng **partial synchrony model** với randomized timeouts để vượt qua giới hạn FLP.

### 1.3 Mục Tiêu Thiết Kế của RAFT

RAFT được thiết kế bởi Diego Ongaro và John Ousterhout (Stanford, 2014) với mục tiêu chính là **understandability** - dễ hiểu hơn Paxos trong khi vẫn đảm bảo các tính chất an toàn tương đương.

**Nguyên tắc thiết kế:**
1. **Decomposition**: Chia bài toán thành các bài toán con độc lập
2. **State space reduction**: Giảm số trạng thái cần xem xét
3. **Strong leadership**: Một leader duy nhất đơn giản hóa log replication

---

## 2. Nền Tảng Lý Thuyết

### 2.1 Mô Hình Lỗi (Failure Model)

RAFT giả định **crash-stop failure model**:

| Loại Lỗi | Định Nghĩa | RAFT Hỗ Trợ |
|----------|------------|-------------|
| Crash-stop | Tiến trình dừng và không bao giờ phục hồi | ✓ |
| Crash-recovery | Tiến trình có thể crash và restart | ✓ (với persistent state) |
| Omission | Mất message | ✓ (RPC retry) |
| Byzantine | Tiến trình có thể hành động tùy ý/độc hại | ✗ |

**Giả định về mạng:**
- Không giả định đồng bộ hoàn toàn (asynchronous)
- Partial synchrony: Tồn tại GST (Global Stabilization Time) sau đó hệ thống đồng bộ
- Không mất message vĩnh viễn (messages có thể delay nhưng cuối cùng được gửi)

### 2.2 Replicated State Machine (RSM)

RAFT triển khai mô hình **Replicated State Machine**:

```
┌─────────────────────────────────────────────────────────────────────┐
│               REPLICATED STATE MACHINE MODEL                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Client Request: cmd                                                │
│         │                                                            │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   CONSENSUS MODULE                           │   │
│  │  (RAFT: Log Replication + Leader Election)                  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│         │                                                            │
│         │ Committed Log Entry                                       │
│         ▼                                                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   STATE MACHINE                              │   │
│  │  σ' = δ(σ, cmd)   where:                                    │   │
│  │  • σ  = current state                                       │   │
│  │  • σ' = new state                                           │   │
│  │  • δ  = deterministic transition function                   │   │
│  └─────────────────────────────────────────────────────────────┘   │
│         │                                                            │
│         ▼                                                            │
│  Response to Client                                                  │
│                                                                      │
│  ═══════════════════════════════════════════════════════════════   │
│  TÍNH CHẤT: Nếu tất cả replicas bắt đầu ở cùng trạng thái và       │
│  apply cùng chuỗi commands theo cùng thứ tự, chúng sẽ đạt          │
│  cùng trạng thái cuối cùng.                                        │
└─────────────────────────────────────────────────────────────────────┘
```

**Định lý RSM (State Machine Safety):**
> Cho state machine S với hàm chuyển đổi xác định δ và trạng thái khởi tạo σ₀.
> Nếu ∀i: logᵢ[1..k] = log[1..k] (tất cả replicas có cùng k entries đầu tiên)
> Thì: δ*(σ₀, log[1..k]) cho kết quả giống nhau trên tất cả replicas.

### 2.3 Đồng Thuận và Linearizability

RAFT đảm bảo **Linearizability** (hay Atomic Consistency) - mạnh nhất trong các consistency models:

```
                    CONSISTENCY MODELS HIERARCHY

                         Linearizability
                              ▲
                              │ (mạnh hơn)
                    Sequential Consistency
                              ▲
                              │
                    Causal Consistency
                              ▲
                              │
                    Eventual Consistency
                              ▲
                              │ (yếu hơn)
```

**Định nghĩa Linearizability:**
- Mọi operation xuất hiện như thể thực thi tại một thời điểm duy nhất giữa invocation và response
- Real-time order được bảo toàn

---

## 3. Mô Hình Hệ Thống

### 3.1 Kiến Trúc Tổng Quan

```
┌─────────────────────────────────────────────────────────────────────┐
│                    RAFT CLUSTER ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│                          ┌─────────┐                                │
│                          │ CLIENT  │                                │
│                          └────┬────┘                                │
│                               │ ClientRequest RPC                   │
│                               ▼                                     │
│     ┌─────────────────────────────────────────────────────────┐    │
│     │                    RAFT CLUSTER                          │    │
│     │                                                          │    │
│     │   ┌─────────┐     ┌─────────┐     ┌─────────┐           │    │
│     │   │ Server₁ │◄───►│ Server₂ │◄───►│ Server₃ │           │    │
│     │   │(Follower)     │ (LEADER)│     │(Follower)│           │    │
│     │   └────┬────┘     └────┬────┘     └────┬────┘           │    │
│     │        │               │               │                 │    │
│     │        │    ┌─────────┐│   ┌─────────┐│                 │    │
│     │        │◄──►│ Server₄ │◄──►│ Server₅ │                  │    │
│     │             │(Follower)│   │(Follower)│                  │    │
│     │             └─────────┘    └─────────┘                   │    │
│     │                                                          │    │
│     │  Communication: gRPC over TCP                            │    │
│     │  RPCs: RequestVote, AppendEntries, ClientRequest         │    │
│     └──────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 Trạng Thái Node

Mỗi server trong RAFT duy trì các trạng thái sau:

#### 3.2.1 Persistent State (Phải lưu trữ bền vững)

| Biến | Kiểu | Mô Tả |
|------|------|-------|
| `currentTerm` | int64 | Term mới nhất server đã thấy (khởi tạo = 0) |
| `votedFor` | string | candidateId đã vote trong term hiện tại (null nếu chưa) |
| `log[]` | []LogEntry | Log entries; mỗi entry chứa command và term |

**Lý do cần persistent:**
- `currentTerm`: Ngăn vote cho nhiều candidates trong cùng term sau restart
- `votedFor`: Đảm bảo mỗi server chỉ vote một lần mỗi term
- `log[]`: Đảm bảo committed entries không bị mất

#### 3.2.2 Volatile State (Tất cả servers)

| Biến | Kiểu | Mô Tả |
|------|------|-------|
| `commitIndex` | int64 | Index của log entry cao nhất đã commit (khởi tạo = 0) |
| `lastApplied` | int64 | Index của log entry cao nhất đã apply vào state machine (khởi tạo = 0) |

#### 3.2.3 Volatile State (Chỉ Leader)

| Biến | Kiểu | Mô Tả |
|------|------|-------|
| `nextIndex[]` | map[server]int64 | Với mỗi server, index của entry tiếp theo cần gửi (khởi tạo = leader last log index + 1) |
| `matchIndex[]` | map[server]int64 | Với mỗi server, index của entry cao nhất đã replicate (khởi tạo = 0) |

### 3.3 Cấu Trúc Log Entry

```
┌─────────────────────────────────────────────────────────────────────┐
│                        LOG ENTRY STRUCTURE                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   LogEntry {                                                        │
│       Term:    int64   // Term khi entry được leader tạo           │
│       Index:   int64   // Vị trí trong log (1-indexed)             │
│       Command: string  // State machine command                     │
│   }                                                                  │
│                                                                      │
│   Ví dụ log:                                                        │
│   ┌───────┬───────┬───────┬───────┬───────┬───────┬───────┐        │
│   │ idx=1 │ idx=2 │ idx=3 │ idx=4 │ idx=5 │ idx=6 │ idx=7 │        │
│   ├───────┼───────┼───────┼───────┼───────┼───────┼───────┤        │
│   │term=1 │term=1 │term=1 │term=2 │term=3 │term=3 │term=3 │        │
│   ├───────┼───────┼───────┼───────┼───────┼───────┼───────┤        │
│   │SET x=3│SET y=1│SET y=9│SET x=2│SET x=0│SET y=7│SET x=5│        │
│   └───────┴───────┴───────┴───────┴───────┴───────┴───────┘        │
│                                   ▲               ▲                 │
│                            commitIndex=4    lastIndex=7             │
│                                                                      │
│   [  COMMITTED & APPLIED  ][    UNCOMMITTED - REPLICATED    ]       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. Thuật Toán RAFT Chi Tiết

### 4.1 Tổng Quan Các Thành Phần

RAFT được phân rã thành ba bài toán con tương đối độc lập:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    RAFT ALGORITHM COMPONENTS                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ 1. LEADER ELECTION                                            │ │
│  │    • Khi nào bắt đầu election?                               │ │
│  │    • Làm sao để bầu leader mới?                              │ │
│  │    • Làm sao để phát hiện leader failure?                    │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                              │                                      │
│                              ▼                                      │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ 2. LOG REPLICATION                                            │ │
│  │    • Leader tiếp nhận commands từ clients                    │ │
│  │    • Leader replicate entries đến followers                  │ │
│  │    • Khi nào entry được coi là committed?                    │ │
│  │    • Xử lý log inconsistencies như thế nào?                  │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                              │                                      │
│                              ▼                                      │
│  ┌───────────────────────────────────────────────────────────────┐ │
│  │ 3. SAFETY                                                     │ │
│  │    • Đảm bảo chỉ một leader mỗi term                         │ │
│  │    • Đảm bảo logs cuối cùng nhất quán                        │ │
│  │    • Đảm bảo state machine safety                            │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.2 Leader Election

#### 4.2.1 Khái Niệm Term

**Term** là đơn vị thời gian logic trong RAFT, đóng vai trò như **logical clock**:

```
Time ──────────────────────────────────────────────────────────────────►

  Term 1       Term 2      Term 3       Term 4           Term 5
├──────────┼───────────┼───┼───────────────────────┼───────────────►
│          │           │   │                       │
│ Election │ Election  │ No│      Election         │   Election
│   ↓      │    ↓      │Ldr│         ↓             │      ↓
│ Leader 1 │  Leader 2 │   │      Leader 3         │   Leader 4
│          │           │   │                       │
│ Normal   │  Normal   │   │      Normal           │   Normal
│ Operation│ Operation │   │     Operation         │  Operation

Mỗi term bắt đầu với election.
Term có thể không có leader (split vote) → chuyển sang term tiếp theo.
```

**Tính chất của Term:**
1. Mỗi server lưu trữ `currentTerm` tăng đơn điệu
2. Servers trao đổi term trong mọi RPC
3. Nếu nhận được term cao hơn → cập nhật và chuyển thành follower
4. Nếu nhận được request với term thấp hơn → reject

#### 4.2.2 Election Timeout và Randomization

**Election timeout** là khoảng thời gian follower chờ trước khi bắt đầu election nếu không nhận được communication từ leader.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ELECTION TIMEOUT DESIGN                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Timeout được chọn ngẫu nhiên trong khoảng [T, 2T]                  │
│  Trong triển khai này: [150ms, 300ms]                               │
│                                                                      │
│  Server 1: |----timeout=187ms----|→ Start Election                  │
│  Server 2: |------timeout=234ms------|→ (Already voted)             │
│  Server 3: |----------timeout=289ms----------|→ (Leader elected)    │
│                                                                      │
│  ═══════════════════════════════════════════════════════════════   │
│                                                                      │
│  Xác suất split vote:                                               │
│                                                                      │
│  P(split vote) ≈ (broadcast time / election timeout)^(n/2)          │
│                                                                      │
│  Với timeout=150-300ms và RTT~10ms:                                 │
│  P(split vote) < 0.3^2.5 ≈ 0.05 cho cluster 5 nodes                │
│                                                                      │
│  Lý do chọn timeout >> heartbeat interval:                          │
│  • Heartbeat = 50ms (trong triển khai này)                          │
│  • Election timeout = 150-300ms                                      │
│  • Tỷ lệ 3-6x cho phép nhiều heartbeats trước khi election          │
│  • Giảm false positives do network delay                            │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.2.3 RequestVote RPC

**Pseudocode chi tiết:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                      RequestVote RPC                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Arguments:                                                          │
│    term         : candidate's term                                  │
│    candidateId  : candidate requesting vote                         │
│    lastLogIndex : index of candidate's last log entry              │
│    lastLogTerm  : term of candidate's last log entry               │
│                                                                      │
│  Results:                                                            │
│    term         : currentTerm, for candidate to update itself      │
│    voteGranted  : true means candidate received vote               │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Receiver Implementation (Xử lý tại voter):                         │
│                                                                      │
│  1. IF term < currentTerm:                                          │
│        RETURN (currentTerm, false)                                  │
│                                                                      │
│  2. IF term > currentTerm:                                          │
│        currentTerm ← term                                           │
│        state ← Follower                                             │
│        votedFor ← null                                              │
│                                                                      │
│  3. IF votedFor ∈ {null, candidateId}                              │
│     AND candidate's log is at least as up-to-date:                 │
│                                                                      │
│        votedFor ← candidateId                                       │
│        Reset election timer                                         │
│        RETURN (currentTerm, true)                                   │
│                                                                      │
│  4. ELSE:                                                            │
│        RETURN (currentTerm, false)                                  │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Log Up-to-date Check (§5.4.1 trong paper):                        │
│                                                                      │
│  Candidate's log is "at least as up-to-date" if:                   │
│    lastLogTerm > receiver's lastLogTerm                            │
│    OR                                                                │
│    (lastLogTerm == receiver's lastLogTerm                          │
│     AND lastLogIndex >= receiver's lastLogIndex)                   │
│                                                                      │
│  Ý nghĩa: Entry với term cao hơn là mới hơn. Nếu cùng term,        │
│           log dài hơn là mới hơn.                                   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.2.4 Election Algorithm

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ELECTION ALGORITHM                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  CANDIDATE PROCEDURE (khi election timeout):                        │
│                                                                      │
│  1. currentTerm++                                                   │
│  2. state ← Candidate                                               │
│  3. votedFor ← self                                                 │
│  4. votesReceived ← 1                                               │
│  5. Reset election timer (random 150-300ms)                         │
│                                                                      │
│  6. FOR EACH server s IN peers (parallel):                          │
│        response ← RequestVote(                                      │
│            term = currentTerm,                                      │
│            candidateId = self,                                      │
│            lastLogIndex = len(log),                                 │
│            lastLogTerm = log[lastIndex].term                        │
│        )                                                             │
│                                                                      │
│        IF response.term > currentTerm:                              │
│            currentTerm ← response.term                              │
│            state ← Follower                                         │
│            votedFor ← null                                          │
│            RETURN                                                    │
│                                                                      │
│        IF response.voteGranted:                                     │
│            votesReceived++                                          │
│            IF votesReceived > len(peers)/2:                         │
│                state ← Leader                                       │
│                Initialize nextIndex, matchIndex                     │
│                Start sending heartbeats                             │
│                RETURN                                                │
│                                                                      │
│  7. IF election timeout expires and still Candidate:                │
│        Start new election (go to step 1)                            │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.2.5 State Transition Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                   STATE TRANSITION DIAGRAM                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│                      Starts here                                    │
│                          │                                          │
│                          ▼                                          │
│                   ┌──────────────┐                                  │
│                   │   FOLLOWER   │◄──────────────────────┐          │
│                   └──────┬───────┘                       │          │
│                          │                               │          │
│        Election timeout, │                    Discovers  │          │
│        no heartbeat      │                    current    │          │
│        received          │                    leader or  │          │
│                          │                    higher term│          │
│                          ▼                               │          │
│                   ┌──────────────┐                       │          │
│        ┌─────────│  CANDIDATE   │────────────────────────┤          │
│        │         └──────┬───────┘                        │          │
│        │                │                                │          │
│        │   Election     │ Receives                       │          │
│        │   timeout,     │ majority                       │          │
│        │   new election │ votes                          │          │
│        │                │                                │          │
│        │                ▼                                │          │
│        │         ┌──────────────┐                        │          │
│        └────────►│    LEADER    │────────────────────────┘          │
│                  └──────────────┘                                   │
│                         │                                           │
│                         │ Discovers server                          │
│                         │ with higher term                          │
│                         │                                           │
│                         ▼                                           │
│                   ┌──────────────┐                                  │
│                   │   FOLLOWER   │                                  │
│                   └──────────────┘                                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.3 Log Replication

#### 4.3.1 AppendEntries RPC

```
┌─────────────────────────────────────────────────────────────────────┐
│                     AppendEntries RPC                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Arguments:                                                          │
│    term         : leader's term                                     │
│    leaderId     : so follower can redirect clients                  │
│    prevLogIndex : index of log entry immediately preceding          │
│                   new entries                                       │
│    prevLogTerm  : term of prevLogIndex entry                        │
│    entries[]    : log entries to store (empty for heartbeat)        │
│    leaderCommit : leader's commitIndex                              │
│                                                                      │
│  Results:                                                            │
│    term         : currentTerm, for leader to update itself          │
│    success      : true if follower contained entry matching         │
│                   prevLogIndex and prevLogTerm                      │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Receiver Implementation:                                            │
│                                                                      │
│  1. IF term < currentTerm:                                          │
│        RETURN (currentTerm, false)                                  │
│                                                                      │
│  2. IF term >= currentTerm:                                         │
│        currentTerm ← term                                           │
│        state ← Follower                                             │
│        leaderId ← leaderId                                          │
│        Reset election timer                                         │
│                                                                      │
│  3. IF log doesn't contain entry at prevLogIndex                    │
│     with term == prevLogTerm:                                       │
│        RETURN (currentTerm, false)                                  │
│                                                                      │
│  4. IF existing entry conflicts with new entry                      │
│     (same index, different terms):                                  │
│        Delete the existing entry and all that follow                │
│                                                                      │
│  5. Append any new entries not already in log                       │
│                                                                      │
│  6. IF leaderCommit > commitIndex:                                  │
│        commitIndex ← min(leaderCommit, index of last new entry)    │
│                                                                      │
│  7. RETURN (currentTerm, true)                                      │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.3.2 Log Consistency Check

**Log Matching Property** đảm bảo bởi inductive argument:

```
┌─────────────────────────────────────────────────────────────────────┐
│                  LOG MATCHING PROPERTY                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Nếu hai logs chứa entry với cùng index và term, thì:              │
│  (1) Entry đó lưu cùng command                                      │
│  (2) Logs giống hệt nhau cho tất cả entries trước đó               │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Chứng minh bằng quy nạp:                                           │
│                                                                      │
│  Base case (index = 1):                                             │
│    • Leader tạo tối đa một entry với index cố định trong term      │
│    • Followers không bao giờ thay đổi index của entry              │
│    → Entry đầu tiên với term t tại index 1 là duy nhất             │
│                                                                      │
│  Inductive step:                                                     │
│    Giả sử tính chất đúng cho entries 1..k-1                        │
│    Xét entry tại index k:                                           │
│                                                                      │
│    • AppendEntries consistency check đảm bảo:                       │
│      entry[k] chỉ được append nếu entry[k-1] match                 │
│    • Theo giả thiết quy nạp: nếu entry[k-1] match                  │
│      thì entries 1..k-1 giống hệt nhau                             │
│    • Leader chỉ tạo một entry với mỗi index trong term             │
│    → entry[k] là duy nhất và logs 1..k giống nhau                  │
│                                                                      │
│  ∎ (Q.E.D.)                                                         │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.3.3 Log Inconsistency và Recovery

```
┌─────────────────────────────────────────────────────────────────────┐
│              LOG INCONSISTENCY SCENARIOS                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Leader (term 8) log:                                               │
│  ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐                 │
│  │ 1 │ 1 │ 1 │ 4 │ 4 │ 5 │ 5 │ 6 │ 6 │ 6 │ 7 │ 7 │                 │
│  └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘                 │
│    1   2   3   4   5   6   7   8   9  10  11  12   ← index          │
│                                                                      │
│  Possible follower logs:                                            │
│                                                                      │
│  (a) ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐                 │
│      │ 1 │ 1 │ 1 │ 4 │ 4 │ 5 │ 5 │ 6 │ 6 │ 6 │   │ Missing entries │
│      └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘                 │
│                                                                      │
│  (b) ┌───┬───┬───┬───┐                                              │
│      │ 1 │ 1 │ 1 │ 4 │                     Missing many entries    │
│      └───┴───┴───┴───┘                                              │
│                                                                      │
│  (c) ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐             │
│      │ 1 │ 1 │ 1 │ 4 │ 4 │ 5 │ 5 │ 6 │ 6 │ 6 │ 6 │ 6 │ Extra entries│
│      └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘ (uncommitted)│
│                                                                      │
│  (d) ┌───┬───┬───┬───┬───┬───┬───┐                                  │
│      │ 1 │ 1 │ 1 │ 4 │ 4 │ 4 │ 4 │   Conflicting entries           │
│      └───┴───┴───┴───┴───┴───┴───┘   (different term at index 6)   │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  RECOVERY MECHANISM:                                                 │
│                                                                      │
│  Leader duy trì nextIndex[follower] cho mỗi follower               │
│                                                                      │
│  1. Khởi tạo: nextIndex[f] = leader's lastLogIndex + 1              │
│                                                                      │
│  2. Gửi AppendEntries với:                                          │
│     prevLogIndex = nextIndex[f] - 1                                 │
│     prevLogTerm = log[prevLogIndex].term                            │
│                                                                      │
│  3. Nếu follower reject (consistency check fail):                   │
│     nextIndex[f]--                                                  │
│     Retry AppendEntries                                             │
│                                                                      │
│  4. Nếu follower accept:                                            │
│     matchIndex[f] = prevLogIndex + len(entries)                     │
│     nextIndex[f] = matchIndex[f] + 1                                │
│                                                                      │
│  Optimization: Follower có thể trả về conflicting entry info       │
│  để leader skip back nhanh hơn (không bắt buộc cho correctness)    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

#### 4.3.4 Commit Rule

```
┌─────────────────────────────────────────────────────────────────────┐
│                      COMMIT RULE                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Entry được coi là COMMITTED khi:                                   │
│                                                                      │
│  1. Entry được replicated trên majority (quorum)                    │
│  2. Entry thuộc term hiện tại của leader                            │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  QUAN TRỌNG: Điều kiện (2) là cần thiết!                            │
│                                                                      │
│  Ví dụ vì sao cần điều kiện (2):                                    │
│                                                                      │
│  Thời điểm t1 (Leader = S1, term = 2):                              │
│                                                                      │
│  S1: │1│2│     S1 replicate entry term=2 tại index 2 đến S2        │
│  S2: │1│2│                                                          │
│  S3: │1│       S3 chưa nhận được                                    │
│  S4: │1│                                                            │
│  S5: │1│                                                            │
│                                                                      │
│  Thời điểm t2 (S1 crash, S5 becomes leader, term = 3):              │
│                                                                      │
│  S1: │1│2│     (crashed)                                            │
│  S2: │1│2│                                                          │
│  S3: │1│3│     S5 replicate entry term=3 (với quorum S3,S4,S5)     │
│  S4: │1│3│                                                          │
│  S5: │1│3│                                                          │
│                                                                      │
│  → Entry term=2 tại index 2 (trên S1,S2) bị override!              │
│  → Nếu S1 đã commit entry đó → vi phạm safety!                     │
│                                                                      │
│  Giải pháp: Leader chỉ commit entries từ term hiện tại             │
│             Entries từ terms trước được commit gián tiếp            │
│             (khi entry term hiện tại được commit, tất cả entries   │
│              trước đó cũng được commit theo Log Matching Property)  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.4 Heartbeat Mechanism

```
┌─────────────────────────────────────────────────────────────────────┐
│                    HEARTBEAT MECHANISM                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Leader gửi AppendEntries RPC định kỳ (mỗi 50ms trong triển khai)  │
│                                                                      │
│  Mục đích:                                                          │
│  1. Duy trì authority - ngăn followers timeout và start election   │
│  2. Replicate new entries (nếu có)                                  │
│  3. Update followers' commitIndex                                   │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Timeline:                                                          │
│                                                                      │
│  Leader:  ──●────────●────────●────────●────────●────────●────────►│
│             │        │        │        │        │        │   time  │
│             │50ms    │50ms    │50ms    │50ms    │50ms    │         │
│             │        │        │        │        │        │         │
│             ▼        ▼        ▼        ▼        ▼        ▼         │
│  Follower:  ●        ●        ●        ●        ●        ●         │
│             │        │        │        │        │        │         │
│             Reset    Reset    Reset    Reset    Reset    Reset     │
│             Timer    Timer    Timer    Timer    Timer    Timer     │
│                                                                      │
│  Follower's election timeout: 150-300ms                             │
│  → Follower nhận ~3-6 heartbeats trước khi timeout                 │
│  → Robust against occasional network delays                        │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Heartbeat vs. Full AppendEntries:                                  │
│                                                                      │
│  Heartbeat:              Full AppendEntries:                        │
│  entries[] = empty       entries[] = new entries                    │
│  Chỉ reset timer         Reset timer + replicate log               │
│  Thường xuyên (50ms)     Khi có new commands                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 5. Đảm Bảo Tính Đúng Đắn

### 5.1 Safety Properties

RAFT đảm bảo năm tính chất safety:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    SAFETY PROPERTIES                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. ELECTION SAFETY                                                  │
│     ━━━━━━━━━━━━━━━━━━                                              │
│     "At most one leader can be elected in a given term"             │
│                                                                      │
│     Chứng minh:                                                      │
│     • Mỗi server vote tối đa một lần mỗi term (votedFor)           │
│     • Candidate cần majority votes để thắng                         │
│     • Hai majorities luôn giao nhau ít nhất 1 server               │
│     → Không thể có hai leaders trong cùng term ∎                   │
│                                                                      │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                      │
│  2. LEADER APPEND-ONLY                                               │
│     ━━━━━━━━━━━━━━━━━━━                                             │
│     "A leader never overwrites or deletes entries in its log;       │
│      it only appends new entries"                                   │
│                                                                      │
│     Đảm bảo bởi: Leader algorithm chỉ append, không modify         │
│                                                                      │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                      │
│  3. LOG MATCHING                                                     │
│     ━━━━━━━━━━━━━━                                                  │
│     "If two logs contain an entry with the same index and term,    │
│      then the logs are identical in all entries up through         │
│      the given index"                                               │
│                                                                      │
│     Chứng minh: Xem mục 4.3.2                                       │
│                                                                      │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                      │
│  4. LEADER COMPLETENESS                                              │
│     ━━━━━━━━━━━━━━━━━━━━                                            │
│     "If a log entry is committed in a given term, then that        │
│      entry will be present in the logs of the leaders for all      │
│      higher-numbered terms"                                         │
│                                                                      │
│     Chứng minh (sketch):                                            │
│     • Entry E committed tại term T → replicated trên majority M₁   │
│     • Leader term T' > T phải nhận votes từ majority M₂            │
│     • M₁ ∩ M₂ ≠ ∅ (majority intersection)                          │
│     • Voter trong intersection có E                                 │
│     • Vote restriction: candidate phải có log ≥ voter              │
│     → Leader term T' phải có E ∎                                   │
│                                                                      │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                      │
│  5. STATE MACHINE SAFETY                                             │
│     ━━━━━━━━━━━━━━━━━━━━━━                                          │
│     "If a server has applied a log entry at a given index to       │
│      its state machine, no other server will ever apply a          │
│      different log entry for the same index"                        │
│                                                                      │
│     Chứng minh:                                                      │
│     Từ Log Matching + Leader Completeness:                          │
│     • Committed entry tại index i là unique                         │
│     • Tất cả servers cuối cùng sẽ có cùng entry tại index i        │
│     → Apply cùng entry cho index i ∎                               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.2 Liveness Properties

```
┌─────────────────────────────────────────────────────────────────────┐
│                    LIVENESS PROPERTIES                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  RAFT đảm bảo liveness under partial synchrony:                     │
│                                                                      │
│  Timing requirement cho election:                                   │
│                                                                      │
│  broadcastTime << electionTimeout << MTBF                           │
│                                                                      │
│  Trong đó:                                                          │
│  • broadcastTime: thời gian gửi RPC đến tất cả servers             │
│  • electionTimeout: 150-300ms trong triển khai này                  │
│  • MTBF: Mean Time Between Failures                                 │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Giải thích:                                                        │
│                                                                      │
│  1. broadcastTime << electionTimeout                                │
│     Đảm bảo leader có thể gửi heartbeats trước khi followers       │
│     timeout. Ngăn unnecessary elections.                            │
│                                                                      │
│  2. electionTimeout << MTBF                                         │
│     Đảm bảo hệ thống có đủ thời gian hoạt động bình thường         │
│     giữa các failures. Ngăn elections liên tục.                     │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Progress guarantee:                                                 │
│                                                                      │
│  Nếu:                                                               │
│  1. Majority servers operational                                    │
│  2. Network eventually delivers messages                            │
│  3. Timing requirement được thỏa mãn                                │
│                                                                      │
│  Thì: Hệ thống cuối cùng sẽ elect leader và make progress          │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.3 Invariants

```
┌─────────────────────────────────────────────────────────────────────┐
│                      SYSTEM INVARIANTS                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Các bất biến phải luôn đúng trong quá trình thực thi:             │
│                                                                      │
│  I1. Election Invariant                                             │
│      ∀ term t: |{s : s.state = Leader ∧ s.currentTerm = t}| ≤ 1   │
│      (Tối đa một leader mỗi term)                                   │
│                                                                      │
│  I2. Log Invariant                                                   │
│      ∀ s₁, s₂, i:                                                   │
│        (s₁.log[i].term = s₂.log[i].term)                           │
│        → (s₁.log[1..i] = s₂.log[1..i])                             │
│      (Entries với cùng index và term → logs prefix giống nhau)     │
│                                                                      │
│  I3. Commit Invariant                                                │
│      ∀ s: s.commitIndex ≤ len(s.log)                               │
│      (commitIndex không vượt quá log length)                        │
│                                                                      │
│  I4. Apply Invariant                                                 │
│      ∀ s: s.lastApplied ≤ s.commitIndex                            │
│      (Chỉ apply entries đã commit)                                  │
│                                                                      │
│  I5. Term Monotonicity                                               │
│      Trong mọi execution: s.currentTerm chỉ tăng                   │
│      (Term không bao giờ giảm)                                      │
│                                                                      │
│  I6. Vote Uniqueness                                                 │
│      ∀ term t, server s:                                            │
│        s votes at most once trong term t                            │
│                                                                      │
│  I7. Leader Log Superset                                             │
│      Nếu s là leader của term t và entry E committed trước term t: │
│        E ∈ s.log                                                    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 6. Phân Tích Độ Phức Tạp

### 6.1 Message Complexity

```
┌─────────────────────────────────────────────────────────────────────┐
│                   MESSAGE COMPLEXITY                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Cho cluster với N nodes:                                           │
│                                                                      │
│  Operation              │ Messages  │ Notes                         │
│  ───────────────────────┼───────────┼─────────────────────────────  │
│  Leader Election        │ O(N)      │ N-1 RequestVote RPCs          │
│  Single Command Commit  │ O(N)      │ N-1 AppendEntries RPCs        │
│  Heartbeat (per round)  │ O(N)      │ N-1 AppendEntries RPCs        │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Throughput Analysis:                                               │
│                                                                      │
│  • Leader bottleneck: Leader phải xử lý tất cả client requests     │
│  • Network bound: Mỗi command cần O(N) messages                    │
│  • Batching possible: Nhiều commands có thể gộp trong một round    │
│                                                                      │
│  Latency:                                                            │
│                                                                      │
│  • Best case: 1 RTT (leader local + majority ack)                  │
│  • Commit latency: time until majority replicate + 1 heartbeat     │
│  • Apply latency: commit latency + apply time                      │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 Storage Complexity

```
┌─────────────────────────────────────────────────────────────────────┐
│                   STORAGE COMPLEXITY                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Per-server storage:                                                │
│                                                                      │
│  Component        │ Space         │ Description                     │
│  ─────────────────┼───────────────┼───────────────────────────────  │
│  log[]            │ O(K × M)      │ K entries, M bytes/entry        │
│  currentTerm      │ O(1)          │ Single integer                  │
│  votedFor         │ O(1)          │ Single string                   │
│  commitIndex      │ O(1)          │ Single integer                  │
│  lastApplied      │ O(1)          │ Single integer                  │
│  nextIndex[]      │ O(N)          │ One per peer (leader only)     │
│  matchIndex[]     │ O(N)          │ One per peer (leader only)     │
│  kvStore          │ O(S)          │ State machine state             │
│                                                                      │
│  Total: O(K × M + N + S)                                            │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Log compaction (không triển khai trong project này):              │
│                                                                      │
│  • Snapshotting: Định kỳ snapshot state machine                    │
│  • Discard log: Xóa entries đã apply trong snapshot                │
│  • InstallSnapshot RPC: Gửi snapshot cho lagging followers         │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.3 Time Complexity

```
┌─────────────────────────────────────────────────────────────────────┐
│                    TIME COMPLEXITY                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Operation                    │ Time                                │
│  ─────────────────────────────┼────────────────────────────────     │
│  Process RequestVote          │ O(1)                                │
│  Process AppendEntries        │ O(E) where E = entries count       │
│  Start Election               │ O(N) parallel RPCs                  │
│  Send Heartbeats              │ O(N) parallel RPCs                  │
│  Advance Commit Index         │ O(N) to check matchIndex           │
│  Apply Entry                  │ O(1) assuming O(1) state machine   │
│  Find Inconsistent Entry      │ O(log length) worst case           │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Election Convergence Time:                                         │
│                                                                      │
│  Expected time to elect leader:                                     │
│  E[T_election] ≈ electionTimeout + broadcastTime                   │
│                                                                      │
│  With randomized timeout [T, 2T]:                                   │
│  P(split vote) ≈ (broadcastTime / T)^(N/2)                         │
│                                                                      │
│  For T=150ms, broadcastTime=10ms, N=5:                              │
│  P(split vote) ≈ (10/150)^2.5 ≈ 0.003                              │
│                                                                      │
│  → Expected ~1.003 election rounds                                  │
│  → E[T_election] ≈ 160-310ms                                        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 7. So Sánh Với Các Thuật Toán Khác

### 7.1 RAFT vs Paxos

```
┌─────────────────────────────────────────────────────────────────────┐
│                     RAFT vs PAXOS                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Aspect            │ RAFT              │ Paxos                      │
│  ──────────────────┼───────────────────┼────────────────────────    │
│  Leader            │ Strong leader     │ Weak/no leader             │
│                    │ required          │ (Multi-Paxos has leader)   │
│                    │                   │                            │
│  Log ordering      │ Entries ordered   │ Entries can have gaps     │
│                    │ consecutively     │                            │
│                    │                   │                            │
│  Leader election   │ Integrated        │ Separate algorithm         │
│                    │                   │                            │
│  Understandability │ Designed for      │ Notoriously difficult      │
│                    │ clarity           │ to understand              │
│                    │                   │                            │
│  Phases per entry  │ 2 phases          │ 2+ phases (Prepare,       │
│                    │ (replicate,       │ Accept, Learn)             │
│                    │  commit)          │                            │
│                    │                   │                            │
│  Safety            │ Equivalent        │ Equivalent                 │
│                    │                   │                            │
│  Performance       │ Similar           │ Similar (with Multi-Paxos) │
│                                                                      │
│  ────────────────────────────────────────────────────────────────   │
│                                                                      │
│  Key Insight:                                                       │
│                                                                      │
│  RAFT decompose bài toán thành:                                     │
│  1. Leader election                                                 │
│  2. Log replication                                                 │
│  3. Safety                                                          │
│                                                                      │
│  Trong khi Paxos giải quyết như một bài toán tổng thể              │
│  → RAFT dễ hiểu và implement hơn                                   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 7.2 RAFT vs Viewstamped Replication

```
┌─────────────────────────────────────────────────────────────────────┐
│              RAFT vs VIEWSTAMPED REPLICATION                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Aspect               │ RAFT          │ VR                          │
│  ─────────────────────┼───────────────┼─────────────────────────    │
│  Term concept         │ term          │ view-number                 │
│  Leader name          │ leader        │ primary                     │
│  Log entries          │ Indexed       │ op-number                   │
│  Election trigger     │ Timeout       │ View-change protocol        │
│  Commit notification  │ Via heartbeat │ Explicit commit messages   │
│                                                                      │
│  Similarities:                                                       │
│  • Both use strong leader                                           │
│  • Both require majority for progress                               │
│  • Both have similar safety properties                              │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 7.3 RAFT vs ZAB (Zookeeper)

```
┌─────────────────────────────────────────────────────────────────────┐
│                      RAFT vs ZAB                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Aspect               │ RAFT              │ ZAB                     │
│  ─────────────────────┼───────────────────┼─────────────────────    │
│  Designed for         │ General consensus │ Zookeeper specifically  │
│  Recovery             │ Log-based         │ Snapshot + txn log      │
│  Election             │ Integrated        │ Separate Fast LE        │
│  Ordering             │ Total order       │ Total order + Zxid      │
│  Configuration        │ Membership change │ Dynamic reconfig        │
│                                                                      │
│  ZAB specific:                                                       │
│  • Zxid = (epoch, counter) for ordering                             │
│  • Primary order: Primary processes writes in FIFO                  │
│  • Different recovery mechanism                                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 8. Chi Tiết Triển Khai

### 8.1 Cấu Trúc Source Code

```
lab1_2.2/
├── proto/
│   └── raft.proto           # gRPC service definition
├── raft/
│   ├── node.go              # Node struct, initialization, helpers
│   ├── election.go          # startElection, RequestVote handler
│   ├── replication.go       # sendHeartbeats, AppendEntries handler
│   └── handlers.go          # ClientRequest, GetStatus handlers
├── cmd/
│   ├── node/main.go         # Node server entry point
│   └── client/main.go       # CLI client
├── scripts/
│   ├── start_cluster.sh     # Start 5-node cluster
│   ├── stop_cluster.sh      # Stop cluster
│   └── test_scenarios.sh    # Test scenarios
└── Makefile                 # Build automation
```

### 8.2 Cấu Hình Timing

```go
// Trong raft/node.go
ElectionTimeoutMin: 150 * time.Millisecond  // T
ElectionTimeoutMax: 300 * time.Millisecond  // 2T
HeartbeatInterval:  50 * time.Millisecond   // << T

// Timing constraint: heartbeat << election timeout
// 50ms << 150-300ms ✓
// Cho phép ~3-6 heartbeats trước election timeout
```

### 8.3 Goroutine Model

```
┌─────────────────────────────────────────────────────────────────────┐
│                    GOROUTINE MODEL                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Main Goroutine                                                      │
│      │                                                               │
│      ├──► gRPC Server (handles incoming RPCs)                       │
│      │                                                               │
│      ├──► connectToPeers()                                          │
│      │       └──► Per-peer connection goroutine                     │
│      │                                                               │
│      ├──► Election Timer (via time.AfterFunc)                       │
│      │       └──► startElection() when timeout                      │
│      │               └──► Per-peer RequestVote goroutine            │
│      │                                                               │
│      ├──► sendHeartbeats() (Leader only)                            │
│      │       └──► Per-peer AppendEntries goroutine                  │
│      │                                                               │
│      └──► applyCommittedEntries()                                   │
│              └──► Periodically apply committed entries              │
│                                                                      │
│  Synchronization:                                                    │
│  • sync.RWMutex for node state (mu)                                 │
│  • sync.RWMutex for kvStore (kvStoreMu)                             │
│  • sync.RWMutex for partitioned map (partitionedMu)                 │
│  • sync.RWMutex for peer connections (peerConnsMu)                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 8.4 Protocol Buffer Definition

```protobuf
// proto/raft.proto

service RaftService {
    // RAFT core RPCs
    rpc RequestVote(RequestVoteRequest) returns (RequestVoteResponse);
    rpc AppendEntries(AppendEntriesRequest) returns (AppendEntriesResponse);

    // Client operations
    rpc ClientRequest(ClientRequestMessage) returns (ClientRequestResponse);

    // Monitoring & testing
    rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
    rpc Partition(PartitionRequest) returns (PartitionResponse);
    rpc Heal(HealRequest) returns (HealResponse);
}

message LogEntry {
    int64 term = 1;
    int64 index = 2;
    string command = 3;  // "SET key value" or "DELETE key"
}

message RequestVoteRequest {
    int64 term = 1;
    string candidate_id = 2;
    int64 last_log_index = 3;
    int64 last_log_term = 4;
}

message AppendEntriesRequest {
    int64 term = 1;
    string leader_id = 2;
    int64 prev_log_index = 3;
    int64 prev_log_term = 4;
    repeated LogEntry entries = 5;
    int64 leader_commit = 6;
}
```

---

## 9. Thực Nghiệm và Đánh Giá

### 9.1 Kịch Bản Test

#### Test 1: Leader Election

```bash
# Khởi động cluster
make run-cluster
sleep 3

# Kiểm tra leader được bầu
make client
> status
# Kỳ vọng: Một node là leader, còn lại là follower
```

#### Test 2: Leader Failure và Re-election

```bash
# Xác định leader hiện tại
> status
[node2] *leader term=1 ...

# Kill leader
pkill -f "node2"

# Chờ election timeout (150-300ms)
sleep 1

# Kiểm tra leader mới
> status
# Kỳ vọng: Leader mới với term cao hơn
```

#### Test 3: Log Replication

```bash
> set x 1
> set y 2
> set z 3
> status
# Kỳ vọng: Tất cả nodes có log length = 4
```

#### Test 4: Network Partition (Majority survives)

```bash
# Partition: {1,2} vs {3,4,5}
> partition localhost:5001 localhost:5003,localhost:5004,localhost:5005
> partition localhost:5002 localhost:5003,localhost:5004,localhost:5005
> partition localhost:5003 localhost:5001,localhost:5002
> partition localhost:5004 localhost:5001,localhost:5002
> partition localhost:5005 localhost:5001,localhost:5002

> status
# Kỳ vọng: Majority (3,4,5) có leader, minority (1,2) không

# Test write trên majority
> set partition_key value
# Kỳ vọng: Thành công

# Heal và kiểm tra convergence
> heal localhost:5001
> heal localhost:5002
> heal localhost:5003
> heal localhost:5004
> heal localhost:5005
```

### 9.2 Metrics Quan Sát

| Metric | Mô Tả | Cách Đo |
|--------|-------|---------|
| Election time | Thời gian từ leader fail đến leader mới | Log timestamps |
| Commit latency | Thời gian từ client request đến commit | Client response time |
| Log sync time | Thời gian follower catch up | matchIndex progression |
| Split vote rate | Tỷ lệ election không thành công | Count term increments |

---

## 10. Hướng Dẫn Sử Dụng

### 10.1 Cài Đặt

```bash
# Yêu cầu: Go 1.21+, protoc

# Install protobuf plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# Build project
make deps
make build
```

### 10.2 Chạy Cluster

```bash
# Khởi động 5-node cluster
make run-cluster

# Kết nối client
make client

# Dừng cluster
make stop-cluster
```

### 10.3 Client Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| SET | `set <key> <value>` | Store key-value pair |
| GET | `get <key>` | Retrieve value |
| DELETE | `delete <key>` | Remove key |
| STATUS | `status [addr]` | View cluster/node status |
| PARTITION | `partition <node> <addrs>` | Simulate network partition |
| HEAL | `heal <node>` | Restore connections |
| QUIT | `quit` | Exit client |

### 10.4 Xem Logs

```bash
# Xem log của node cụ thể
tail -f logs/node1.log

# Follow tất cả logs
make follow-logs
```

---

## 11. Tài Liệu Tham Khảo

### Papers

1. **Ongaro, D., & Ousterhout, J. (2014)**. "In Search of an Understandable Consensus Algorithm". In *USENIX Annual Technical Conference (ATC '14)*. [Link](https://raft.github.io/raft.pdf)

2. **Lamport, L. (1998)**. "The Part-Time Parliament". *ACM Transactions on Computer Systems*, 16(2), 133-169.

3. **Fischer, M. J., Lynch, N. A., & Paterson, M. S. (1985)**. "Impossibility of Distributed Consensus with One Faulty Process". *Journal of the ACM*, 32(2), 374-382.

4. **Liskov, B., & Cowling, J. (2012)**. "Viewstamped Replication Revisited". *MIT CSAIL Technical Report*.

### Online Resources

- RAFT Visualization: https://raft.github.io/
- RAFT Paper Extended Version: https://raft.github.io/raft.pdf
- gRPC Documentation: https://grpc.io/docs/languages/go/
- Protocol Buffers: https://protobuf.dev/

### Thuật Ngữ

| Term | Definition |
|------|------------|
| Consensus | Agreement on a single value among distributed processes |
| Term | Logical time period in RAFT, incremented at each election |
| Log | Ordered sequence of commands to be applied to state machine |
| Commit | Entry durably stored on majority and safe to apply |
| Quorum | Minimum number of nodes (majority) needed for agreement |
| Split Vote | Election where no candidate receives majority |
| Heartbeat | Periodic empty AppendEntries to maintain leadership |

---

*Tài liệu này được viết cho mục đích học thuật - Triển khai thuật toán đồng thuận RAFT trong Go*
