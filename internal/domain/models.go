package domain

type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
)

type Post struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Excerpt        string     `json:"excerpt"`
	Content        string     `json:"content"`
	CategorySlug   string     `json:"categorySlug"`
	Status         PostStatus `json:"status"`
	Featured       bool       `json:"featured"`
	Views          int        `json:"views"`
	Claps          int        `json:"claps,omitempty"`
	Author         string     `json:"author"`
	UpdatedAt      string     `json:"updatedAt"`
	CreatedAt      string     `json:"createdAt"`
	SeoTitle       string     `json:"seoTitle,omitempty"`
	SeoDescription string     `json:"seoDescription,omitempty"`
	FocusKeyword   string     `json:"focusKeyword,omitempty"`
	CanonicalUrl   string     `json:"canonicalUrl,omitempty"`
	NoIndex        bool       `json:"noIndex,omitempty"`
}

type Category struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	ParentSlug  *string `json:"parentSlug"`
	Description string  `json:"description"`
}

type CommentItem struct {
	ID        string `json:"id"`
	PostSlug  string `json:"postSlug"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

type MediaItem struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	URL        string `json:"url"`
	SizeBytes  int64  `json:"sizeBytes"`
	MimeType   string `json:"mimeType"`
	Alt        string `json:"alt"`
	UploadedAt string `json:"uploadedAt"`
}

type AuditLog struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Actor     string `json:"actor"`
	Timestamp string `json:"timestamp"`
	Meta      string `json:"meta,omitempty"`
}

type SiteSettings struct {
	SiteName              string `json:"siteName"`
	Tagline               string `json:"tagline"`
	PostsPerPage          int    `json:"postsPerPage"`
	MaintenanceMode       bool   `json:"maintenanceMode"`
	AllowComments         bool   `json:"allowComments"`
	DefaultSeoDescription string `json:"defaultSeoDescription"`
	SiteURL               string `json:"siteUrl,omitempty"`
	TwitterHandle         string `json:"twitterHandle,omitempty"`
}

type DatabaseStore struct {
	Posts      []Post        `json:"posts"`
	Categories []Category    `json:"categories"`
	Comments   []CommentItem `json:"comments,omitempty"`
	Media      []MediaItem   `json:"media"`
	AuditLogs  []AuditLog    `json:"auditLogs"`
	Settings   SiteSettings  `json:"settings"`
}

type DashboardStats struct {
	TotalPosts     int `json:"totalPosts"`
	PublishedPosts int `json:"publishedPosts"`
	DraftPosts     int `json:"draftPosts"`
	ArchivedPosts  int `json:"archivedPosts"`
	TotalViews     int `json:"totalViews"`
	TotalClaps     int `json:"totalClaps"`
	TotalComments  int `json:"totalComments"`
	TotalCategories int `json:"totalCategories"`
}
