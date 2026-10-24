package chaptervcs

// ==================== 存储后端抽象 ====================
// 默认后端：自研版本链（快照文件 + DB 元数据），零外部依赖，契合既有「章节正文落文件」架构。
// 可选后端：真实 git 仓库（go-git），需以 `-tags gitvcs` 构建，运行时由 NVS_VCS_BACKEND=git 启用；
// 启用失败自动回落默认后端，不影响启动。
//
// 上层（handlers / 业务代码）一律调用包级函数，不感知后端差异。

import (
	"errors"
	"os"

	"nvs-server/models"
)

// Backend 章节版本存储后端
type Backend interface {
	Name() string

	Commit(in CommitInput) (*ChapterVersion, *Conflict, error)
	List(chapterID uint, branch string) ([]ChapterVersion, error)
	Get(chapterID uint, branch string, version int) (*ChapterVersion, error)
	Head(chapterID uint, branch string) int
	Content(ownerID, novelID uint, v *ChapterVersion) (string, error)
	Revert(ownerID, novelID uint, chapter *models.Chapter, target *ChapterVersion, authorID uint) (*ChapterVersion, error)
	DeleteVersion(ownerID, novelID, chapterID uint, branch string, version int) error
	Branches(chapterID uint) ([]ChapterBranch, error)
	CreateBranch(ownerID, novelID uint, chapter *models.Chapter, name, fromBranch string, fromVer int, authorID uint) (*ChapterVersion, error)
	Merge(ownerID, novelID uint, chapter *models.Chapter, fromBranch, intoBranch string, authorID uint, message string) (*MergeResult, error)
}

// Current 当前生效后端（默认自研版本链）
var Current Backend = defaultBackend{}

// defaultBackend 自研版本链（包内已有的实现）
type defaultBackend struct{}

func (defaultBackend) Name() string { return "chain" }

func (defaultBackend) Commit(in CommitInput) (*ChapterVersion, *Conflict, error) {
	return commitDefault(in)
}
func (defaultBackend) List(chapterID uint, branch string) ([]ChapterVersion, error) {
	return listDefault(chapterID, branch)
}
func (defaultBackend) Get(chapterID uint, branch string, version int) (*ChapterVersion, error) {
	return getDefault(chapterID, branch, version)
}
func (defaultBackend) Head(chapterID uint, branch string) int {
	return headDefault(chapterID, branch)
}
func (defaultBackend) Content(ownerID, novelID uint, v *ChapterVersion) (string, error) {
	return contentDefault(ownerID, novelID, v)
}
func (defaultBackend) Revert(ownerID, novelID uint, chapter *models.Chapter, target *ChapterVersion, authorID uint) (*ChapterVersion, error) {
	return revertDefault(ownerID, novelID, chapter, target, authorID)
}
func (defaultBackend) DeleteVersion(ownerID, novelID, chapterID uint, branch string, version int) error {
	return deleteVersionDefault(ownerID, novelID, chapterID, branch, version)
}
func (defaultBackend) Branches(chapterID uint) ([]ChapterBranch, error) {
	return branchesDefault(chapterID)
}
func (defaultBackend) CreateBranch(ownerID, novelID uint, chapter *models.Chapter, name, fromBranch string, fromVer int, authorID uint) (*ChapterVersion, error) {
	return createBranchDefault(ownerID, novelID, chapter, name, fromBranch, fromVer, authorID)
}
func (defaultBackend) Merge(ownerID, novelID uint, chapter *models.Chapter, fromBranch, intoBranch string, authorID uint, message string) (*MergeResult, error) {
	return mergeDefault(ownerID, novelID, chapter, fromBranch, intoBranch, authorID, message)
}

// ─── 包级入口（业务侧统一调用，自动走 Current 后端） ───

func Commit(in CommitInput) (*ChapterVersion, *Conflict, error) { return Current.Commit(in) }

func List(chapterID uint, branch string) ([]ChapterVersion, error) {
	return Current.List(chapterID, branch)
}

func Get(chapterID uint, branch string, version int) (*ChapterVersion, error) {
	return Current.Get(chapterID, branch, version)
}

func Head(chapterID uint, branch string) int { return Current.Head(chapterID, branch) }

func HeadVersion(chapterID uint, branch string) (*ChapterVersion, error) {
	ver := Head(chapterID, branch)
	if ver <= 0 {
		return nil, ErrNoVersion
	}
	return Get(chapterID, NormalizeBranch(branch), ver)
}

func Content(ownerID, novelID uint, v *ChapterVersion) (string, error) {
	return Current.Content(ownerID, novelID, v)
}

func Revert(ownerID, novelID uint, chapter *models.Chapter, target *ChapterVersion, authorID uint) (*ChapterVersion, error) {
	return Current.Revert(ownerID, novelID, chapter, target, authorID)
}

func DeleteVersion(ownerID, novelID, chapterID uint, branch string, version int) error {
	return Current.DeleteVersion(ownerID, novelID, chapterID, branch, version)
}

func Branches(chapterID uint) ([]ChapterBranch, error) { return Current.Branches(chapterID) }

func CreateBranch(ownerID, novelID uint, chapter *models.Chapter, name, fromBranch string, fromVer int, authorID uint) (*ChapterVersion, error) {
	return Current.CreateBranch(ownerID, novelID, chapter, name, fromBranch, fromVer, authorID)
}

func Merge(ownerID, novelID uint, chapter *models.Chapter, fromBranch, intoBranch string, authorID uint, message string) (*MergeResult, error) {
	return Current.Merge(ownerID, novelID, chapter, fromBranch, intoBranch, authorID, message)
}

// ─── 后端切换 ───

// SetBackend 按名称切换后端（chain / git）
func SetBackend(name string) error {
	switch name {
	case "", "chain", "default":
		Current = defaultBackend{}
		return nil
	case "git":
		b, err := newGitBackend()
		if err != nil {
			return err
		}
		Current = b
		return nil
	default:
		return errors.New("未知的版本控制后端：" + name)
	}
}

// Init 启动时按环境变量 NVS_VCS_BACKEND 选择后端；失败回落默认后端（不阻断启动）
func Init() {
	name := os.Getenv("NVS_VCS_BACKEND")
	if name == "" || name == "chain" {
		return
	}
	if err := SetBackend(name); err != nil {
		Current = defaultBackend{}
		return
	}
}
