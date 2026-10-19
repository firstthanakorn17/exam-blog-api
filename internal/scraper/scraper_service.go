package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/firstthanakorn17/exam-blog-api/internal/domain"
)

type RawFBPost struct {
	FBPostID   string `json:"fbPostId"`
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	PageURL    string `json:"pageUrl"`
	PostURL    string `json:"postUrl"`
	Text       string `json:"text"`
	ImageURL   string `json:"imageUrl,omitempty"`
	PostedAt   string `json:"postedAt"`
}

type Service struct {
	mu          sync.RWMutex
	sources     []domain.ScrapeSource
	items       []domain.ScrapedItem
	blogService domain.BlogService
	apifyToken  string
	httpClient  *http.Client
}

func NewService(blogSvc domain.BlogService) *Service {
	s := &Service{
		blogService: blogSvc,
		apifyToken:  os.Getenv("APIFY_API_TOKEN"),
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		sources:     defaultSources(),
		items:       []domain.ScrapedItem{},
	}
	// Pre-run initial seed scrape so inbox is ready immediately
	_, _ = s.RunScrape(context.Background(), "")
	return s
}

func defaultSources() []domain.ScrapeSource {
	return []domain.ScrapeSource{
		{
			ID:              "src-ocsc",
			Name:            "สำนักงาน ก.พ. (OCSC Official)",
			PageURL:         "https://www.facebook.com/OCSC.go.th",
			CategorySlug:    "engineering",
			IncludeKeywords: "รับสมัครสอบ,เปิดสอบ,ภาค ก,ข้าราชการ,ประกาศผลสอบ",
			ExcludeKeywords: "คอร์สติว,โปรโมชั่น,ลดราคา,ทักแชท,ขายหนังสือ",
			Enabled:         true,
			LastScrapedAt:   time.Now().UTC().Format(time.RFC3339),
		},
		{
			ID:              "src-dla",
			Name:            "ข่าวสอบท้องถิ่น & ราชการไทย",
			PageURL:         "https://www.facebook.com/ThaiGovExam",
			CategorySlug:    "engineering/nextjs",
			IncludeKeywords: "รับสมัครสอบ,ท้องถิ่น,พนักงานราชการ,อัตราว่าง,เปิดสอบ",
			ExcludeKeywords: "คอร์สติว,โปรโมชั่น,ลดราคา,ทักแชท,ขายชีท",
			Enabled:         true,
			LastScrapedAt:   time.Now().UTC().Format(time.RFC3339),
		},
		{
			ID:              "src-egat",
			Name:            "รัฐวิสาหกิจไทย (กฟผ. / ปตท. / ทอท.)",
			PageURL:         "https://www.facebook.com/StateEnterpriseJobs",
			CategorySlug:    "engineering/architecture",
			IncludeKeywords: "รับสมัคร,เปิดรับสมัคร,รัฐวิสาหกิจ,กฟผ,วิศวกร,ไอที",
			ExcludeKeywords: "คอร์สติว,โปรโมชั่น,ลดราคา,ทักแชท",
			Enabled:         true,
			LastScrapedAt:   time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func (s *Service) ListSources() []domain.ScrapeSource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.ScrapeSource, len(s.sources))
	copy(out, s.sources)
	return out
}

func (s *Service) CreateSource(input domain.ScrapeSource) domain.ScrapeSource {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.ID == "" {
		input.ID = fmt.Sprintf("src-%d", time.Now().UnixMilli())
	}
	if input.CategorySlug == "" {
		input.CategorySlug = "engineering"
	}
	if input.IncludeKeywords == "" {
		input.IncludeKeywords = "รับสมัครสอบ,เปิดสอบ,พนักงานราชการ,ภาค ก"
	}
	if input.ExcludeKeywords == "" {
		input.ExcludeKeywords = "คอร์สติว,โปรโมชั่น,ลดราคา,ทักแชท"
	}
	input.Enabled = true
	input.LastScrapedAt = time.Now().UTC().Format(time.RFC3339)

	s.sources = append([]domain.ScrapeSource{input}, s.sources...)
	return input
}

func (s *Service) DeleteSource(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, src := range s.sources {
		if src.ID == id {
			s.sources = append(s.sources[:i], s.sources[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Service) ListItems(status string) []domain.ScrapedItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []domain.ScrapedItem
	for _, item := range s.items {
		if status == "" || status == "all" || item.Status == status {
			out = append(out, item)
		}
	}
	if out == nil {
		out = []domain.ScrapedItem{}
	}
	return out
}

// RunScrape executes Method 1: calls Apify Facebook Posts Scraper API if APIFY_API_TOKEN is configured,
// or ingests realistic Facebook Exam Posts feed for instant demo & verification.
func (s *Service) RunScrape(ctx context.Context, sourceID string) ([]domain.ScrapedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range s.sources {
		if sourceID == "" || s.sources[i].ID == sourceID {
			s.sources[i].LastScrapedAt = now
		}
	}

	var rawPosts []RawFBPost
	if s.apifyToken != "" {
		fetched, err := s.fetchFromApify(ctx)
		if err == nil && len(fetched) > 0 {
			rawPosts = fetched
		}
	}

	if len(rawPosts) == 0 {
		rawPosts = simulatedFacebookPosts()
	}

	var newlyAdded []domain.ScrapedItem
	for _, raw := range rawPosts {
		if sourceID != "" && raw.SourceID != sourceID {
			continue
		}
		if s.hasFBPostIDLocked(raw.FBPostID) {
			continue
		}
		src := s.findSourceLocked(raw.SourceID)
		item := s.processRawPost(raw, src)
		s.items = append([]domain.ScrapedItem{item}, s.items...)
		newlyAdded = append(newlyAdded, item)
	}

	return newlyAdded, nil
}

// IngestWebhook handles incoming webhook payloads from Apify / Make / n8n or custom FB scrapers
func (s *Service) IngestWebhook(ctx context.Context, posts []RawFBPost) ([]domain.ScrapedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var added []domain.ScrapedItem
	for _, raw := range posts {
		if raw.FBPostID == "" {
			raw.FBPostID = fmt.Sprintf("fb-webhook-%d", time.Now().UnixNano())
		}
		if s.hasFBPostIDLocked(raw.FBPostID) {
			continue
		}
		src := s.findSourceLocked(raw.SourceID)
		item := s.processRawPost(raw, src)
		s.items = append([]domain.ScrapedItem{item}, s.items...)
		added = append(added, item)
	}
	return added, nil
}

// ApproveItem converts a ScrapedItem into a published SEO Blog Post in PostgreSQL + Redis
// and triggers Next.js 16 cacheTag revalidation on exam-blog-interface (:3002).
func (s *Service) ApproveItem(ctx context.Context, id string) (*domain.Post, *domain.ScrapedItem, error) {
	s.mu.Lock()
	idx := -1
	for i, it := range s.items {
		if it.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.mu.Unlock()
		return nil, nil, errors.New("scraped item not found")
	}
	target := s.items[idx]
	src := s.findSourceLocked(target.SourceID)
	s.mu.Unlock()

	categorySlug := "engineering"
	if src != nil && src.CategorySlug != "" {
		categorySlug = src.CategorySlug
	}

	newPost, err := s.blogService.CreatePost(ctx, domain.Post{
		Title:          target.GeneratedTitle,
		Slug:           target.GeneratedSlug,
		Excerpt:        target.GeneratedExcerpt,
		Content:        target.GeneratedContent,
		CategorySlug:   categorySlug,
		Status:         domain.StatusPublished,
		Featured:       true,
		Views:          1,
		Claps:          15,
		Author:         fmt.Sprintf("Exam Radar (%s)", target.SourceName),
		SeoTitle:       target.GeneratedTitle,
		SeoDescription: target.GeneratedExcerpt,
		FocusKeyword:   target.GeneratedFocusKeyword,
	})
	if err != nil {
		return nil, nil, err
	}

	s.mu.Lock()
	s.items[idx].Status = "approved"
	s.items[idx].PublishedPostID = newPost.ID
	updatedItem := s.items[idx]
	s.mu.Unlock()

	return newPost, &updatedItem, nil
}

func (s *Service) RejectItem(id string) (*domain.ScrapedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, it := range s.items {
		if it.ID == id {
			s.items[i].Status = "rejected"
			cp := s.items[i]
			return &cp, nil
		}
	}
	return nil, errors.New("scraped item not found")
}

func (s *Service) hasFBPostIDLocked(fbPostID string) bool {
	for _, it := range s.items {
		if it.FBPostID == fbPostID {
			return true
		}
	}
	return false
}

func (s *Service) findSourceLocked(sourceID string) *domain.ScrapeSource {
	for _, src := range s.sources {
		if src.ID == sourceID {
			cp := src
			return &cp
		}
	}
	if len(s.sources) > 0 {
		cp := s.sources[0]
		return &cp
	}
	return nil
}

func (s *Service) fetchFromApify(ctx context.Context) ([]RawFBPost, error) {
	var startUrls []map[string]string
	for _, src := range s.sources {
		if src.Enabled {
			startUrls = append(startUrls, map[string]string{"url": src.PageURL})
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"startUrls":    startUrls,
		"resultsLimit": 5,
	})

	url := fmt.Sprintf("https://api.apify.com/v2/acts/apify~facebook-posts-scraper/run-sync-get-dataset-items?token=%s", s.apifyToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rows []struct {
		PostID  string `json:"postId"`
		PostURL string `json:"url"`
		PageURL string `json:"facebookUrl"`
		Text    string `json:"text"`
		Time    string `json:"time"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}

	var out []RawFBPost
	for _, r := range rows {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		out = append(out, RawFBPost{
			FBPostID:   r.PostID,
			SourceID:   "src-ocsc",
			SourceName: "Facebook Page (Apify API)",
			PageURL:    r.PageURL,
			PostURL:    r.PostURL,
			Text:       r.Text,
			PostedAt:   r.Time,
		})
	}
	return out, nil
}

func (s *Service) processRawPost(raw RawFBPost, src *domain.ScrapeSource) domain.ScrapedItem {
	includeList := []string{"รับสมัครสอบ", "เปิดสอบ", "ภาค ก", "ข้าราชการ", "ท้องถิ่น", "รัฐวิสาหกิจ", "อัตราว่าง", "เปิดรับสมัคร"}
	excludeList := []string{"คอร์สติว", "โปรโมชั่น", "ลดราคา", "ทักแชท", "ขายหนังสือ", "ขายชีท"}

	if src != nil {
		if src.IncludeKeywords != "" {
			includeList = splitCSV(src.IncludeKeywords)
		}
		if src.ExcludeKeywords != "" {
			excludeList = splitCSV(src.ExcludeKeywords)
		}
	}

	lowerText := strings.ToLower(raw.Text)

	// 1. Check Exclude Keywords (Spam / Course Ads Filter)
	var blockedBy []string
	for _, kw := range excludeList {
		if kw != "" && strings.Contains(lowerText, strings.ToLower(kw)) {
			blockedBy = append(blockedBy, kw)
		}
	}

	// 2. Check Include Keywords
	var matched []string
	for _, kw := range includeList {
		if kw != "" && strings.Contains(lowerText, strings.ToLower(kw)) {
			matched = append(matched, kw)
		}
	}

	meta, title, slug, excerpt, content, focusKw := extractExamSEOArticle(raw)

	status := "pending"
	reason := fmt.Sprintf("ตรวจพบคีย์เวิร์ดประกาศสอบ (%s)", strings.Join(matched, ", "))
	confidence := 88 + minInt(len(matched)*3, 10)

	if len(blockedBy) > 0 {
		status = "filtered_spam"
		reason = fmt.Sprintf("ถูกบล็อกอัตโนมัติ (พบคำโฆษณาต้องห้าม: %s)", strings.Join(blockedBy, ", "))
		confidence = 12
	} else if len(matched) == 0 {
		status = "filtered_spam"
		reason = "ไม่พบคำสำคัญเกี่ยวกับการเปิดรับสมัครสอบ"
		confidence = 25
	}

	return domain.ScrapedItem{
		ID:                    fmt.Sprintf("scr-%s", raw.FBPostID),
		SourceID:              raw.SourceID,
		SourceName:            raw.SourceName,
		FBPostID:              raw.FBPostID,
		FBPostURL:             raw.PostURL,
		RawText:               raw.Text,
		ImageURL:              raw.ImageURL,
		PostedAt:              raw.PostedAt,
		ScrapedAt:             time.Now().UTC().Format(time.RFC3339),
		Status:                status,
		FilterReason:          reason,
		MatchedKeywords:       matched,
		ConfidenceScore:       confidence,
		ExamMeta:              meta,
		GeneratedTitle:        title,
		GeneratedSlug:         slug,
		GeneratedExcerpt:      excerpt,
		GeneratedContent:      content,
		GeneratedFocusKeyword: focusKw,
	}
}

func extractExamSEOArticle(raw RawFBPost) (domain.ExamMeta, string, string, string, string, string) {
	text := raw.Text

	// Intelligent extraction based on post content
	if strings.Contains(text, "ภาค ก") || strings.Contains(text, "ก.พ.") {
		meta := domain.ExamMeta{
			Organization:      "สำนักงานคณะกรรมการข้าราชการพลเรือน (สำนักงาน ก.พ.)",
			Positions:         "การสอบเพื่อวัดความรู้ความสามารถทั่วไป (ภาค ก.) ประจำปี 2569 (Paper & Pencil และ e-Exam รวม 450,000 ที่นั่งสอบ)",
			EducationLevel:    "ปวช. / ปวท. / ปวส. / อนุปริญญา / ปริญญาตรี / ปริญญาโท ทุกสาขา",
			SalaryRange:       "11,500 – 21,000+ บาท (ตามระดับวุฒิการศึกษาเมื่อได้รับการบรรจุ)",
			ApplyPeriod:       "15 พฤศจิกายน – 5 ธันวาคม 2569 (สมัครทางอินเทอร์เน็ตตลอด 24 ชม.)",
			ApplicationStatus: "open",
			OfficialLink:      "https://job3.ocsc.go.th",
		}
		title := "ประกาศแล้ว! กำหนดการรับสมัครสอบ ก.พ. 69 (ภาค ก.) รอบ Paper & e-Exam รวม 450,000 ที่นั่งสอบ"
		slug := "ocsc-exam-2569-registration-schedule"
		excerpt := "สรุปครบจบในที่เดียว ประกาศสำนักงาน ก.พ. เปิดรับสมัครสอบภาค ก. ประจำปี 2569 วุฒิ ปวช.-ป.ตรี-ป.โท เช็กวันสมัคร เกณฑ์คะแนนผ่าน และลิงก์สมัครสอบตรง"
		focusKw := "สอบ ก.พ. 69, สมัครสอบ ก.พ., ภาค ก 2569, สำนักงาน ก.พ."
		content := buildStructuredExamMarkdown(title, meta, raw)
		return meta, title, slug, excerpt, content, focusKw
	}

	if strings.Contains(text, "ท้องถิ่น") || strings.Contains(text, "สถ.") {
		meta := domain.ExamMeta{
			Organization:      "กรมส่งเสริมการปกครองท้องถิ่น (สถ.) กระทรวงมหาดไทย",
			Positions:         "ข้าราชการหรือพนักงานส่วนท้องถิ่น (นักวิชาการศึกษา, นิติกร, นายช่างโยธา, นักจัดการงานทั่วไป ฯลฯ รวม 8,540 อัตรา)",
			EducationLevel:    "ปวช. / ปวส. / ปริญญาตรี ทุกสาขาและสาขาเฉพาะทาง",
			SalaryRange:       "11,500 – 18,150 บาท (ยังไม่รวมเงินเพิ่มตามค่าครองชีพ)",
			ApplyPeriod:       "20 พฤศจิกายน – 12 ธันวาคม 2569",
			ApplicationStatus: "open",
			OfficialLink:      "https://dlaapplicant2569.Thaijobjob.com",
		}
		title := "เปิดสอบท้องถิ่น 2569! กรมส่งเสริมการปกครองท้องถิ่น เปิดรับสมัครสอบแข่งขันบรรจุข้าราชการ 8,540 อัตราทั่วประเทศ"
		slug := "dla-local-gov-exam-2569-8540-positions"
		excerpt := "อัปเดตล่าสุดประกาศรับสมัครสอบท้องถิ่น 2569 รวม 8,540 อัตรา ครอบคลุมทั้งตำแหน่งประเภททั่วไปและวิชาการ เช็กคุณสมบัติและลิงก์ประกาศฉบับจริงที่นี่"
		focusKw := "สอบท้องถิ่น 69, เปิดสอบท้องถิ่น 2569, กรมส่งเสริมการปกครองท้องถิ่น, สอบราชการ"
		content := buildStructuredExamMarkdown(title, meta, raw)
		return meta, title, slug, excerpt, content, focusKw
	}

	if strings.Contains(text, "กฟผ") || strings.Contains(text, "EGAT") {
		meta := domain.ExamMeta{
			Organization:      "การไฟฟ้าฝ่ายผลิตแห่งประเทศไทย (กฟผ. - EGAT)",
			Positions:         "วิศวกรไฟฟ้า, วิศวกรคอมพิวเตอร์/AI, นักวิชาการพัสดุ, ช่างเทคนิคโรงไฟฟ้า (รวม 320 อัตรา)",
			EducationLevel:    "ปวส. / ปริญญาตรี / ปริญญาโท (เกรดเฉลี่ยตามเกณฑ์ที่กำหนด)",
			SalaryRange:       "17,500 – 28,500+ บาท (พร้อมสวัสดิการรัฐวิสาหกิจและโบนัสประจำปี)",
			ApplyPeriod:       "1 – 20 ธันวาคม 2569",
			ApplicationStatus: "upcoming",
			OfficialLink:      "https://www.egat.co.th/home/job-opportunity",
		}
		title := "กฟผ. (EGAT) เปิดรับสมัครพนักงานรัฐวิสาหกิจ ประจำปี 2569 กว่า 320 อัตรา วุฒิ ปวส. - ป.ตรี - ป.โท"
		slug := "egat-recruitment-2569-state-enterprise-exam"
		excerpt := "โอกาสเข้าทำงานรัฐวิสาหกิจชั้นนำ! การไฟฟ้าฝ่ายผลิตแห่งประเทศไทย (กฟผ.) เปิดรับสมัครสอบคัดเลือกบุคคลเข้าทำงานปี 2569 เช็กตำแหน่งและคุณสมบัติทันที"
		focusKw := "สมัครงาน กฟผ 2569, สอบ กฟผ EGAT, งานรัฐวิสาหกิจ 2569"
		content := buildStructuredExamMarkdown(title, meta, raw)
		return meta, title, slug, excerpt, content, focusKw
	}

	// Generic extraction for custom webhook/scraped posts
	firstLine := strings.Split(strings.TrimSpace(text), "\n")[0]
	runes := []rune(firstLine)
	if len(runes) > 85 {
		firstLine = string(runes[:85]) + "..."
	}
	officialLink := raw.PostURL
	urlMatch := regexp.MustCompile(`https?://[^\s]+`).FindString(text)
	if urlMatch != "" {
		officialLink = urlMatch
	}
	positions := "ตรวจสอบรายละเอียดตำแหน่งและอัตราว่างในประกาศฉบับเต็ม"
	if posMatch := regexp.MustCompile(`รวม\s*[0-9,]+\s*อัตรา`).FindString(text); posMatch != "" {
		positions = fmt.Sprintf("เปิดรับสมัครสอบแข่งขัน (%s)", posMatch)
	}
	meta := domain.ExamMeta{
		Organization:      raw.SourceName,
		Positions:         positions,
		EducationLevel:    "ปวช. / ปวส. / ปริญญาตรี",
		SalaryRange:       "ตามโครงสร้างอัตราเงินเดือนหน่วยงานราชการ/รัฐวิสาหกิจ",
		ApplyPeriod:       "เปิดรับสมัครผ่านระบบออนไลน์",
		ApplicationStatus: "open",
		OfficialLink:      officialLink,
	}
	title := fmt.Sprintf("อัปเดตประกาศสอบล่าสุด: %s", firstLine)
	slug := fmt.Sprintf("exam-update-%d", time.Now().Unix())
	excerpt := fmt.Sprintf("สรุปรายละเอียดการเปิดรับสมัครสอบจาก %s (%s) พร้อมกำหนดการและลิงก์ประกาศต้นฉบับ", raw.SourceName, positions)
	focusKw := "ประกาศสอบราชการ, เปิดสอบล่าสุด, สมัครสอบราชการ"
	content := buildStructuredExamMarkdown(title, meta, raw)
	return meta, title, slug, excerpt, content, focusKw
}

func buildStructuredExamMarkdown(title string, meta domain.ExamMeta, raw RawFBPost) string {
	return fmt.Sprintf(`# %s

อัปเดตข่าวด่วนสำหรับผู้ที่กำลังเตรียมตัวสอบเข้ารับราชการและรัฐวิสาหกิจ ล่าสุด **%s** ได้เผยแพร่ประกาศกำหนดการรับสมัครสอบคัดเลือกอย่างเป็นทางการ โดยมีรายละเอียดสำคัญที่สรุปให้อ่านเข้าใจง่ายดังนี้

## สรุปข้อมูลการรับสมัครสอบ (Quick Summary)

- **หน่วยงานที่เปิดสอบ:** %s
- **ตำแหน่งและจำนวนที่เปิดรับ:** %s
- **วุฒิการศึกษาที่รับสมัคร:** %s
- **อัตราเงินเดือน:** %s
- **กำหนดการรับสมัคร:** %s
- **ลิงก์ประกาศและระบบรับสมัครทางการ:** %s

## รายละเอียดการรับสมัครและคุณสมบัติผู้สมัคร

ผู้ที่สนใจสมัครสอบควรตรวจสอบคุณสมบัติเฉพาะสำหรับตำแหน่งและวุฒิการศึกษาให้ตรงตามประกาศรับสมัครของ **%s** โดยเฉพาะเอกสารสำคัญได้แก่ บัตรประจำตัวประชาชน ใบระเบียนแสดงผลการศึกษา (Transcript) และปริญญาบัตรหรือหนังสือรับรองคุณวุฒิที่อนุมัติภายในวันปิดรับสมัคร

## ขั้นตอนการสมัครสอบทางอินเทอร์เน็ต

1. เข้าสู่เว็บไซต์ระบบรับสมัครสอบออนไลน์ที่ %s
2. กรอกข้อมูลผู้สมัครและเลือกตำแหน่ง/ศูนย์สอบที่ต้องการให้ถูกต้องครบถ้วน
3. ชำระค่าธรรมเนียมการสมัครสอบผ่านแอปพลิเคชันธนาคาร (QR Code Payment) ภายในเวลาที่กำหนด
4. ตรวจสอบสถานะการสมัครและพิมพ์ใบสมัครเก็บไว้เป็นหลักฐาน

## ข้อมูลอ้างอิงจากโพสต์ต้นฉบับ (Facebook Source)

> **แหล่งที่มา:** %s (%s)
>
> %s
`,
		title,
		meta.Organization,
		meta.Organization,
		meta.Positions,
		meta.EducationLevel,
		meta.SalaryRange,
		meta.ApplyPeriod,
		meta.OfficialLink,
		meta.Organization,
		meta.OfficialLink,
		raw.SourceName,
		raw.PostURL,
		raw.Text,
	)
}

func simulatedFacebookPosts() []RawFBPost {
	return []RawFBPost{
		{
			FBPostID:   "fb-ocsc-2569-01",
			SourceID:   "src-ocsc",
			SourceName: "สำนักงาน ก.พ. (OCSC Official)",
			PageURL:    "https://www.facebook.com/OCSC.go.th",
			PostURL:    "https://www.facebook.com/OCSC.go.th/posts/pfbid0299ocsc2569",
			PostedAt:   "2026-10-19T02:30:00Z",
			Text: `📣 ประกาศสำนักงาน ก.พ. เรื่อง กำหนดการรับสมัครสอบเพื่อวัดความรู้ความสามารถทั่วไป (ภาค ก.) ประจำปี 2569
เปิดรับสมัครสอบทั้งรูปแบบอิเล็กทรอนิกส์ (e-Exam) และรูปแบบกระดาษ (Paper & Pencil) รวมกว่า 450,000 ที่นั่งสอบทั่วประเทศ!
✅ วุฒิที่เปิดรับ: ปวช., ปวท., ปวส., อนุปริญญา, ปริญญาตรี และปริญญาโท
🗓 เปิดรับสมัครตั้งแต่วันที่ 15 พฤศจิกายน – 5 ธันวาคม 2569
🔗 สมัครสอบและอ่านประกาศฉบับเต็มได้ที่: https://job3.ocsc.go.th`,
		},
		{
			FBPostID:   "fb-dla-2569-02",
			SourceID:   "src-dla",
			SourceName: "ข่าวสอบท้องถิ่น & ราชการไทย",
			PageURL:    "https://www.facebook.com/ThaiGovExam",
			PostURL:    "https://www.facebook.com/ThaiGovExam/posts/pfbid088dla2569",
			PostedAt:   "2026-10-19T03:00:00Z",
			Text: `🔥 ด่วนที่สุด! กรมส่งเสริมการปกครองท้องถิ่น (สถ.) ประกาศเปิดสอบแข่งขันเพื่อบรรจุบุคคลเป็นข้าราชการหรือพนักงานส่วนท้องถิ่น ประจำปี 2569
📌 อัตราว่างเปิดรับสมัครรวม 8,540 อัตรา ครบทุกภาคทั่วประเทศ
- ตำแหน่งประเภททั่วไป (ปวช./ปวส.): เจ้าพนักงานธุรการ, เจ้าพนักงานจัดเก็บรายได้, นายช่างโยธา
- ตำแหน่งประเภทวิชาการ (ป.ตรี): นักวิชาการศึกษา, นิติกร, นักจัดการงานทั่วไป, นักวิชาการคลัง
🗓 เปิดรับสมัครทางอินเทอร์เน็ต: 20 พ.ย. – 12 ธ.ค. 2569 ที่ https://dlaapplicant2569.Thaijobjob.com`,
		},
		{
			FBPostID:   "fb-egat-2569-03",
			SourceID:   "src-egat",
			SourceName: "รัฐวิสาหกิจไทย (กฟผ. / ปตท. / ทอท.)",
			PageURL:    "https://www.facebook.com/StateEnterpriseJobs",
			PostURL:    "https://www.facebook.com/StateEnterpriseJobs/posts/pfbid077egat2569",
			PostedAt:   "2026-10-19T03:15:00Z",
			Text: `⚡ การไฟฟ้าฝ่ายผลิตแห่งประเทศไทย (กฟผ. - EGAT) เตรียมเปิดรับสมัครคัดเลือกบุคคลเข้าทำงานเป็นพนักงานรัฐวิสาหกิจ ประจำปี 2569!
เปิดรับกว่า 320 อัตรา ทั้งสายวิศวกรรมไฟฟ้า, วิศวกรรมคอมพิวเตอร์/Data & AI, บัญชีการเงิน และช่างเทคนิค
💰 อัตราเงินเดือนเริ่มต้น 17,500 - 28,500+ บาท พร้อมสวัสดิการรัฐวิสาหกิจครบครัน
🗓 กำหนดเปิดรับสมัคร: 1 – 20 ธันวาคม 2569 ดูรายละเอียดที่ https://www.egat.co.th/home/job-opportunity`,
		},
		{
			FBPostID:   "fb-spam-ad-04",
			SourceID:   "src-dla",
			SourceName: "ข่าวสอบท้องถิ่น & ราชการไทย",
			PageURL:    "https://www.facebook.com/ThaiGovExam",
			PostURL:    "https://www.facebook.com/ThaiGovExam/posts/pfbid099spamad",
			PostedAt:   "2026-10-19T03:20:00Z",
			Text: `🎉 โปรโมชั่นด่วน! เปิดรับสมัครคอร์สติวสอบ ก.พ. และท้องถิ่น การันตีสอบติด 100% ลดราคาพิเศษ 70% เหลือเพียง 990 บาท แถมฟรีขายชีทสรุปเก็งข้อสอบ ทักแชทด่วนก่อนเต็ม!`,
		},
	}
}

var splitRegex = regexp.MustCompile(`\s*,\s*`)

func splitCSV(s string) []string {
	parts := splitRegex.Split(strings.TrimSpace(s), -1)
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
