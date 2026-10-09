# maidirector

並行して進めている開発の「どのストーリーのどの PR まで進んだか」を1画面で追うための、ローカルで動くダッシュボード。

- ストーリーごとに proto → microservice → gateway → frontend のような PR の並びを並べ、どこで止まっているかを見せる
- GitHub の PR の状態（レビュー・CI・マージ・リリース済み）を gh CLI 経由で取ってくる
- 組織内のオープン PR を拾って、レビュータスクとして自動で積む
- スヌーズ、優先度、リポジトリでの絞り込み

Go のバックエンドに React のフロントエンドを埋め込んだ単一バイナリで、データは SQLite（`~/.maidirector/data.db`）に置く。

## 必要なもの

- Go
- Node.js / npm
- [gh](https://cli.github.com/)（`gh auth login` 済み）

## 使い方

```bash
cp maidirector.yml.sample maidirector.yml   # github.org を自分の組織名にする
make build
./bin/maidirector serve
```

http://localhost:20135 を開く。

開発中は `./dev.sh` でバックエンドとフロントエンドを両方起動できる。設計とこれまでの進め方は `DEVELOPMENT.md` にある。
