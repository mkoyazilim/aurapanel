package git

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type Service struct {
	st    *store.Store
	priv  *privclient.Client
	audit *audit.Service
}

func NewService(st *store.Store, priv *privclient.Client, au *audit.Service) *Service {
	return &Service{
		st:    st,
		priv:  priv,
		audit: au,
	}
}

// Deploy tetikler.
// İlk defa yapılıyorsa "git clone" atar. Daha önce yapılmışsa "git pull" atar.
// Sonrasında deploy_script çalıştırılır.
func (s *Service) Deploy(ctx context.Context, siteID string) error {
	g, err := s.st.GetGitDeployment(ctx, siteID)
	if err != nil {
		return err
	}

	// Klasörde .git olup olmadığını kontrol edelim. 
	// script.op kullanarak basit bir bash script ile kontrol ve pull/clone yapabiliriz.
	// Bash script'i:
	// if [ -d ".git" ]; then
	//     git checkout main
	//     git pull
	// else
	//     git clone URL .
	// fi
	// (Kullanıcı yapılandırmasındaki branch kullanılacak)

	script := fmt.Sprintf(`
set -e
export GIT_TERMINAL_PROMPT=0

if [ -d ".git" ]; then
    echo "Updating repository..."
    git fetch origin
    git checkout %s
    git reset --hard origin/%s
else
    echo "Cloning repository..."
    git clone -b %s %s .
fi

echo "Running deploy script..."
%s
`, g.Branch, g.Branch, g.Branch, g.RepoURL, g.DeployScript)

	scriptB64 := base64.StdEncoding.EncodeToString([]byte(script))
	
	// Deploy_path, home dizinine göredir. (örn: "/")
	cwd := g.DeployPath
	if cwd == "" {
		cwd = "/"
	}

	// Çalıştır
	s.st.UpdateGitStatus(ctx, siteID, "deploying")

	_, err = s.priv.Call(ctx, "script.op", map[string]any{
		"site":       siteID,
		"cwd":        cwd,
		"script_b64": scriptB64,
	})

	if err != nil {
		s.st.UpdateGitStatus(ctx, siteID, "failed")
		s.audit.Write(ctx, audit.Event{
			Action: "git.deploy", Target: siteID, Result: "failed",
		})
		// Burada hata detayını da DB'ye yazabilirdik ama MVP için yeterli.
		return fmt.Errorf("git deploy hatası: %w", err)
	}

	s.st.UpdateGitStatus(ctx, siteID, "success")
	s.audit.Write(ctx, audit.Event{
		Action: "git.deploy", Target: siteID, Result: "success",
	})
	return nil
}
