FROM mrkushalsm/judge0:cgv2
# isolate (cgv2) aceita apenas box_id de 0 a 999; sem % 1000 todas as
# submissões a partir do id 1000 falham com "Sandbox ID out of range".
USER root
RUN sed -i 's/submission\.id%2147483647/submission.id%1000/' /api/app/jobs/isolate_job.rb
USER judge0
